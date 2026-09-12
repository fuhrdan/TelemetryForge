---- MODULE ReplicatedDurability ----
EXTENDS FiniteSets, TLC

Copies == {"origin", "peerA", "peerB"}

VARIABLES durable, clientAck, downstream, released

vars == <<durable, clientAck, downstream, released>>
Quorum == 2

Init ==
    /\ durable = {}
    /\ clientAck = FALSE
    /\ downstream = FALSE
    /\ released = FALSE

PersistOrigin ==
    /\ "origin" \notin durable
    /\ ~released
    /\ durable' = durable \cup {"origin"}
    /\ UNCHANGED <<clientAck, downstream, released>>

Replicate(peer) ==
    /\ peer \in {"peerA", "peerB"}
    /\ "origin" \in durable
    /\ peer \notin durable
    /\ ~released
    /\ durable' = durable \cup {peer}
    /\ UNCHANGED <<clientAck, downstream, released>>

AckClient ==
    /\ Cardinality(durable) >= Quorum
    /\ ~clientAck
    /\ clientAck' = TRUE
    /\ UNCHANGED <<durable, downstream, released>>

Deliver ==
    /\ Cardinality(durable) >= Quorum
    /\ ~downstream
    /\ downstream' = TRUE
    /\ UNCHANGED <<durable, clientAck, released>>

Release ==
    /\ downstream
    /\ ~released
    /\ released' = TRUE
    /\ durable' = {}
    /\ UNCHANGED <<clientAck, downstream>>

Next ==
    PersistOrigin
    \/ (\E peer \in {"peerA", "peerB"}: Replicate(peer))
    \/ AckClient
    \/ Deliver
    \/ Release

Spec == Init /\ [][Next]_vars

TypeOK ==
    /\ durable \subseteq Copies
    /\ clientAck \in BOOLEAN
    /\ downstream \in BOOLEAN
    /\ released \in BOOLEAN

AckedUndeliveredHasQuorum ==
    (clientAck /\ ~downstream) => Cardinality(durable) >= Quorum

QuorumContainsOrigin ==
    (~released /\ Cardinality(durable) >= Quorum) => "origin" \in durable

ReleaseAfterDelivery == released => downstream

====
