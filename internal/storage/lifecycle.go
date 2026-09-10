package storage

import (
    "context"
    "crypto/rand"
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "strings"
    "time"

    "github.com/fuhrdan/TelemetryForge/internal/lifecycle"
    "github.com/jackc/pgx/v5"
)

func lifecycleID(prefix string) string {
    raw := make([]byte, 10)
    if _, err := rand.Read(raw); err != nil { return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()) }
    return prefix + "-" + hex.EncodeToString(raw)
}

func (store *PostgresStore) CreateLifecycleArtifact(ctx context.Context, kind lifecycle.Kind, payload []byte, actor string) (lifecycle.Artifact, error) {
    name, version, digest, err := lifecycle.ValidatePayload(kind, payload)
    if err != nil { return lifecycle.Artifact{}, err }
    actor = strings.TrimSpace(actor); if actor == "" { return lifecycle.Artifact{}, errors.New("actor is required") }
    item := lifecycle.Artifact{ID:lifecycleID("CFG"), Kind:kind, Name:name, Version:version, SHA256:digest, Payload:append(json.RawMessage(nil), payload...), State:lifecycle.StateDraft, CreatedBy:actor, CreatedAt:time.Now().UTC()}
    _, err = store.pool.Exec(ctx, `INSERT INTO configuration_artifacts(artifact_id,kind,name,version,sha256,payload,state,created_by,created_at) VALUES($1,$2,$3,$4,$5,$6::jsonb,$7,$8,$9)`, item.ID,string(kind),name,version,digest,string(payload),string(item.State),actor,item.CreatedAt)
    if err != nil { return lifecycle.Artifact{}, fmt.Errorf("create lifecycle artifact: %w", err) }
    _ = store.auditLifecycle(ctx,item.ID,kind,"create",actor,name+"@"+version)
    return item,nil
}

func (store *PostgresStore) ListLifecycleArtifacts(ctx context.Context, kind lifecycle.Kind, limit int) ([]lifecycle.Artifact,error) {
    if limit < 1 || limit > 500 { limit=100 }
    rows,err:=store.pool.Query(ctx,`SELECT artifact_id,kind,name,version,sha256,state,created_by,created_at,scheduled_for,activated_at,retired_at FROM configuration_artifacts WHERE ($1='' OR kind=$1) ORDER BY created_at DESC LIMIT $2`,string(kind),limit)
    if err!=nil{return nil,fmt.Errorf("list lifecycle artifacts: %w",err)}; defer rows.Close()
    out:=make([]lifecycle.Artifact,0,limit)
    for rows.Next(){var x lifecycle.Artifact;var kindS,state string;if err:=rows.Scan(&x.ID,&kindS,&x.Name,&x.Version,&x.SHA256,&state,&x.CreatedBy,&x.CreatedAt,&x.ScheduledFor,&x.ActivatedAt,&x.RetiredAt);err!=nil{return nil,err};x.Kind=lifecycle.Kind(kindS);x.State=lifecycle.State(state);out=append(out,x)}
    return out,rows.Err()
}

func (store *PostgresStore) LifecycleArtifact(ctx context.Context,id string, includePayload bool)(lifecycle.Artifact,error){
    var x lifecycle.Artifact;var kindS,state string;var raw []byte
    err:=store.pool.QueryRow(ctx,`SELECT artifact_id,kind,name,version,sha256,payload,state,created_by,created_at,scheduled_for,activated_at,retired_at FROM configuration_artifacts WHERE artifact_id=$1`,id).Scan(&x.ID,&kindS,&x.Name,&x.Version,&x.SHA256,&raw,&state,&x.CreatedBy,&x.CreatedAt,&x.ScheduledFor,&x.ActivatedAt,&x.RetiredAt)
    if err!=nil{return x,err};x.Kind=lifecycle.Kind(kindS);x.State=lifecycle.State(state);if includePayload{x.Payload=append(json.RawMessage(nil),raw...)};return x,nil
}

func (store *PostgresStore) SetLifecycleShadow(ctx context.Context,id,actor string) error { return store.transitionLifecycle(ctx,id,lifecycle.StateDraft,lifecycle.StateShadow,actor,nil,"shadow") }

func (store *PostgresStore) ApproveLifecycle(ctx context.Context,id,actor,comment string) error {
    item,err:=store.LifecycleArtifact(ctx,id,false);if err!=nil{return err}
    actor=strings.TrimSpace(actor);if actor==""{return errors.New("actor is required")};if actor==item.CreatedBy{return errors.New("creator cannot self-approve configuration")};if item.State!=lifecycle.StateShadow{return fmt.Errorf("artifact must be shadow before approval; state=%s",item.State)}
    tx,err:=store.pool.Begin(ctx);if err!=nil{return err};defer func(){_ = tx.Rollback(context.Background())}()
    if _,err=tx.Exec(ctx,`INSERT INTO configuration_approvals(artifact_id,actor,comment) VALUES($1,$2,$3) ON CONFLICT(artifact_id) DO UPDATE SET actor=EXCLUDED.actor,comment=EXCLUDED.comment,approved_at=now()`,id,actor,comment);err!=nil{return err}
    if _,err=tx.Exec(ctx,`UPDATE configuration_artifacts SET state='approved' WHERE artifact_id=$1 AND state='shadow'`,id);err!=nil{return err}
    if err=tx.Commit(ctx);err!=nil{return err};return store.auditLifecycle(ctx,id,item.Kind,"approve",actor,comment)
}

func (store *PostgresStore) AddLifecycleEvidence(ctx context.Context,id,typ,reference,result,reason,tenant,actor string)(lifecycle.Evidence,error){
    if err:=lifecycle.ValidateEvidence(result,reason);err!=nil{return lifecycle.Evidence{},err};if strings.TrimSpace(reference)==""||strings.TrimSpace(typ)==""||strings.TrimSpace(actor)==""{return lifecycle.Evidence{},errors.New("type, reference, and actor are required")}
    item,err:=store.LifecycleArtifact(ctx,id,false);if err!=nil{return lifecycle.Evidence{},err}
    e:=lifecycle.Evidence{EvidenceID:lifecycleID("EVD"),ArtifactID:id,Type:typ,Reference:reference,Result:strings.ToLower(result),Reason:reason,TenantID:tenant,RecordedBy:actor,RecordedAt:time.Now().UTC()}
    _,err=store.pool.Exec(ctx,`INSERT INTO configuration_evidence(evidence_id,artifact_id,evidence_type,reference,result,reason,tenant_id,recorded_by,recorded_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`,e.EvidenceID,id,typ,reference,e.Result,reason,tenant,actor,e.RecordedAt);if err!=nil{return lifecycle.Evidence{},err};_ = store.auditLifecycle(ctx,id,item.Kind,"evidence",actor,typ+":"+e.Result+":"+reference);return e,nil
}

func (store *PostgresStore) ScheduleLifecycle(ctx context.Context,id string,at time.Time,actor string) error {
    if at.IsZero()||!at.After(time.Now().UTC()){return errors.New("scheduled activation must be in the future")}
    return store.transitionLifecycle(ctx,id,lifecycle.StateApproved,lifecycle.StateScheduled,actor,&at,"schedule")
}

func (store *PostgresStore) ActivateLifecycle(ctx context.Context,id,actor string) error { return store.activateLifecycle(ctx,id,actor,false) }
func (store *PostgresStore) RollbackLifecycle(ctx context.Context,id,actor string) error { return store.activateLifecycle(ctx,id,actor,true) }

func (store *PostgresStore) activateLifecycle(ctx context.Context,id,actor string,rollback bool) error {
    item,err:=store.LifecycleArtifact(ctx,id,false);if err!=nil{return err}
    if rollback { if item.State!=lifecycle.StateRetired{return errors.New("rollback target must be retired")}
    } else { if item.State!=lifecycle.StateApproved && item.State!=lifecycle.StateScheduled{return fmt.Errorf("artifact must be approved or scheduled; state=%s",item.State)}; if item.State==lifecycle.StateScheduled && item.ScheduledFor!=nil && item.ScheduledFor.After(time.Now().UTC()){return errors.New("scheduled activation time has not arrived")}; if err:=store.lifecycleActivationGates(ctx,id);err!=nil{return err} }
    tx,err:=store.pool.Begin(ctx);if err!=nil{return err};defer func(){_ = tx.Rollback(context.Background())}()
    if _,err=tx.Exec(ctx,`UPDATE configuration_artifacts SET state='retired',retired_at=now() WHERE kind=$1 AND state='active'`,string(item.Kind));err!=nil{return err}
    if _,err=tx.Exec(ctx,`UPDATE configuration_artifacts SET state='active',activated_at=now(),retired_at=NULL,scheduled_for=NULL WHERE artifact_id=$1`,id);err!=nil{return err}
    if err=tx.Commit(ctx);err!=nil{return err};action:="activate";if rollback{action="rollback"};return store.auditLifecycle(ctx,id,item.Kind,action,actor,item.Name+"@"+item.Version)
}

func (store *PostgresStore) lifecycleActivationGates(ctx context.Context,id string) error {
    var approvals int; if err:=store.pool.QueryRow(ctx,`SELECT count(*) FROM configuration_approvals WHERE artifact_id=$1`,id).Scan(&approvals);err!=nil{return err};if approvals<1{return errors.New("activation requires approval")}
    var accepted,failed int; if err:=store.pool.QueryRow(ctx,`SELECT count(*) FILTER (WHERE result IN ('pass','waived')), count(*) FILTER (WHERE result='fail') FROM configuration_evidence WHERE artifact_id=$1`,id).Scan(&accepted,&failed);err!=nil{return err};if failed>0{return errors.New("activation blocked by failed evidence")};if accepted<1{return errors.New("activation requires at least one passing or waived evidence gate")};return nil
}

func (store *PostgresStore) RetireLifecycle(ctx context.Context,id,actor string) error {
    item,err:=store.LifecycleArtifact(ctx,id,false);if err!=nil{return err};if item.State==lifecycle.StateRetired{return nil}
    _,err=store.pool.Exec(ctx,`UPDATE configuration_artifacts SET state='retired',retired_at=now(),scheduled_for=NULL WHERE artifact_id=$1`,id);if err!=nil{return err};return store.auditLifecycle(ctx,id,item.Kind,"retire",actor,"")
}

func (store *PostgresStore) ActivateDueLifecycle(ctx context.Context,actor string,limit int)([]string,error){
    if limit<1||limit>100{limit=20};rows,err:=store.pool.Query(ctx,`SELECT artifact_id FROM configuration_artifacts WHERE state='scheduled' AND scheduled_for<=now() ORDER BY scheduled_for LIMIT $1`,limit);if err!=nil{return nil,err};defer rows.Close();var ids []string;for rows.Next(){var id string;if err:=rows.Scan(&id);err!=nil{return nil,err};ids=append(ids,id)}
    var activated []string;for _,id:=range ids{if err:=store.activateLifecycle(ctx,id,actor,false);err!=nil{return activated,fmt.Errorf("activate due %s: %w",id,err)};activated=append(activated,id)};return activated,nil
}

func (store *PostgresStore) ManagedLifecycleConfig(ctx context.Context,kind lifecycle.Kind,role string)(lifecycle.Artifact,bool,error){
    query:=`SELECT artifact_id,kind,name,version,sha256,payload,state,created_by,created_at,scheduled_for,activated_at,retired_at FROM configuration_artifacts WHERE kind=$1 AND state='active'`
    if role=="shadow"{query=`SELECT artifact_id,kind,name,version,sha256,payload,state,created_by,created_at,scheduled_for,activated_at,retired_at FROM configuration_artifacts WHERE kind=$1 AND state IN ('shadow','approved','scheduled') ORDER BY created_at DESC LIMIT 1`}else if role!="active"{return lifecycle.Artifact{},false,fmt.Errorf("unsupported runtime role %q",role)}
    var x lifecycle.Artifact;var kindS,state string;var raw []byte;err:=store.pool.QueryRow(ctx,query,string(kind)).Scan(&x.ID,&kindS,&x.Name,&x.Version,&x.SHA256,&raw,&state,&x.CreatedBy,&x.CreatedAt,&x.ScheduledFor,&x.ActivatedAt,&x.RetiredAt);if errors.Is(err,pgx.ErrNoRows){return x,false,nil};if err!=nil{return x,false,err};x.Kind=lifecycle.Kind(kindS);x.State=lifecycle.State(state);x.Payload=append(json.RawMessage(nil),raw...);return x,true,nil
}

func (store *PostgresStore) RecordLifecycleRuntimeLoad(ctx context.Context,load lifecycle.RuntimeLoad) error {
    _,err:=store.pool.Exec(ctx,`INSERT INTO configuration_runtime_loads(instance_id,component,kind,role,artifact_id,sha256,loaded_at) VALUES($1,$2,$3,$4,$5,$6,now()) ON CONFLICT(instance_id,component,kind,role) DO UPDATE SET artifact_id=EXCLUDED.artifact_id,sha256=EXCLUDED.sha256,loaded_at=now()`,load.InstanceID,load.Component,string(load.Kind),load.Role,load.ArtifactID,load.SHA256);return err
}

func (store *PostgresStore) LifecycleConvergence(ctx context.Context)([]lifecycle.Convergence,error){
    rows,err:=store.pool.Query(ctx,`WITH desired AS (SELECT kind,CASE WHEN state='active' THEN 'active' ELSE 'shadow' END role,artifact_id,sha256 FROM configuration_artifacts WHERE state IN ('active','shadow','approved','scheduled')) SELECT d.kind,d.role,d.artifact_id,d.sha256,count(r.instance_id),count(r.instance_id) FILTER(WHERE r.sha256=d.sha256) FROM desired d LEFT JOIN configuration_runtime_loads r ON r.kind=d.kind AND r.role=d.role AND r.loaded_at>now()-interval '2 minutes' GROUP BY d.kind,d.role,d.artifact_id,d.sha256 ORDER BY d.kind,d.role`);if err!=nil{return nil,err};defer rows.Close();var out []lifecycle.Convergence;for rows.Next(){var x lifecycle.Convergence;var kind string;if err:=rows.Scan(&kind,&x.Role,&x.DesiredID,&x.DesiredSHA256,&x.LiveInstances,&x.Matching);err!=nil{return nil,err};x.Kind=lifecycle.Kind(kind);x.Converged=x.LiveInstances>0&&x.LiveInstances==x.Matching;out=append(out,x)};return out,rows.Err()
}

func (store *PostgresStore) ListLifecycleAudit(ctx context.Context,limit int)([]lifecycle.AuditEntry,error){if limit<1||limit>500{limit=100};rows,err:=store.pool.Query(ctx,`SELECT audit_id,COALESCE(artifact_id,''),COALESCE(kind,''),action,actor,detail,created_at FROM configuration_audit ORDER BY created_at DESC LIMIT $1`,limit);if err!=nil{return nil,err};defer rows.Close();var out []lifecycle.AuditEntry;for rows.Next(){var x lifecycle.AuditEntry;var kind string;if err:=rows.Scan(&x.AuditID,&x.ArtifactID,&kind,&x.Action,&x.Actor,&x.Detail,&x.CreatedAt);err!=nil{return nil,err};x.Kind=lifecycle.Kind(kind);out=append(out,x)};return out,rows.Err()}
func (store *PostgresStore) PruneLifecycleRuntimeLoads(ctx context.Context,before time.Time)(int64,error){tag,err:=store.pool.Exec(ctx,`DELETE FROM configuration_runtime_loads WHERE loaded_at<$1`,before.UTC());if err!=nil{return 0,err};return tag.RowsAffected(),nil}

func (store *PostgresStore) transitionLifecycle(ctx context.Context,id string,from,to lifecycle.State,actor string,schedule *time.Time,action string) error {item,err:=store.LifecycleArtifact(ctx,id,false);if err!=nil{return err};if item.State!=from{return fmt.Errorf("invalid lifecycle transition %s -> %s",item.State,to)};if !lifecycle.CanTransition(from,to){return fmt.Errorf("transition %s -> %s is not allowed",from,to)};tag,err:=store.pool.Exec(ctx,`UPDATE configuration_artifacts SET state=$2,scheduled_for=$3 WHERE artifact_id=$1 AND state=$4`,id,string(to),schedule,string(from));if err!=nil{return err};if tag.RowsAffected()!=1{return errors.New("lifecycle state changed concurrently")};return store.auditLifecycle(ctx,id,item.Kind,action,actor,"")}
func (store *PostgresStore) auditLifecycle(ctx context.Context,id string,kind lifecycle.Kind,action,actor,detail string) error {_,err:=store.pool.Exec(ctx,`INSERT INTO configuration_audit(artifact_id,kind,action,actor,detail) VALUES($1,$2,$3,$4,$5)`,id,string(kind),action,actor,detail);return err}
