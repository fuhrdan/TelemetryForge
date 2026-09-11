#!/usr/bin/env python3
from __future__ import annotations
import argparse, hashlib, json, os, platform, socket, subprocess, time
from datetime import datetime, timezone
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
def sha256(path: str) -> str:
    h=hashlib.sha256()
    with open(path,'rb') as f:
        for chunk in iter(lambda:f.read(1024*1024),b''): h.update(chunk)
    return h.hexdigest()
def git_commit():
    try:return subprocess.check_output(['git','-C',str(ROOT),'rev-parse','HEAD'],text=True).strip()
    except Exception:return 'unknown'
def now(): return datetime.now(timezone.utc).isoformat().replace('+00:00','Z')
def write(path, scenario, status, started, assertions, evidence, config_paths, measurements=None, notes=None):
    configs=[{'name':name,'sha256':sha256(p)} for name,p in config_paths]
    ev=[]
    for item in evidence:
        x=dict(item); ref=x.get('reference','')
        if ref and Path(ref).is_file(): x['sha256']=sha256(ref); x['bytes']=Path(ref).stat().st_size
        ev.append(x)
    obj={'format':'telemetryforge-operational-proof','format_version':1,'run_id':f'{scenario}-{int(time.time()*1000)}','git_commit':git_commit(),'scenario':scenario,'status':status,'started_at':started,'completed_at':now(),'environment':{'host':socket.gethostname(),'os':platform.system(),'architecture':platform.machine(),'python':platform.python_version(),'cpu_count':str(os.cpu_count() or ''),'total_memory_bytes':str((os.sysconf('SC_PAGE_SIZE')*os.sysconf('SC_PHYS_PAGES')) if hasattr(os,'sysconf') else ''),'ci':os.getenv('CI','false')},'assertions':assertions,'measurements':measurements or [],'evidence':ev,'configuration_fingerprints':configs,'notes':notes or []}
    Path(path).parent.mkdir(parents=True,exist_ok=True); Path(path).write_text(json.dumps(obj,indent=2,allow_nan=False)+'\n'); return obj
def main():
    ap=argparse.ArgumentParser();ap.add_argument('--out',required=True);ap.add_argument('--scenario',required=True);ap.add_argument('--status',choices=['pass','fail','error'],required=True);ap.add_argument('--started-at',required=True);ap.add_argument('--assertion',action='append',default=[]);ap.add_argument('--evidence',action='append',default=[]);ap.add_argument('--config',action='append',default=[]);ap.add_argument('--measurement',action='append',default=[]);ap.add_argument('--note',action='append',default=[]);args=ap.parse_args()
    assertions=[]
    for raw in args.assertion:
        parts=raw.split(':',2);assertions.append({'name':parts[0],'passed':parts[1].lower()=='pass','detail':parts[2] if len(parts)>2 else ''})
    evidence=[]
    for raw in args.evidence:
        parts=raw.split(':',2); evidence.append({'kind':parts[0],'reference':parts[1],'detail':parts[2] if len(parts)>2 else ''})
    configs=[]
    for raw in args.config:
        name,path=raw.split(':',1);configs.append((name,path))
    measurements=[]
    for raw in args.measurement:
        parts=raw.split(':',2); measurements.append({'name':parts[0],'value':float(parts[1]),'unit':parts[2] if len(parts)>2 else ''})
    write(args.out,args.scenario,args.status,args.started_at,assertions,evidence,configs,measurements,args.note)
if __name__=='__main__': main()
