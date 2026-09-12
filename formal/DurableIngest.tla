---- MODULE DurableIngest ----
EXTENDS TLC

VARIABLES localDurable, clientAck, downstream, compacted

vars == <<localDurable, clientAck, downstream, compacted>>

Init ==
    /\ localDurable = FALSE
    /\ clientAck = FALSE
    /\ downstream = FALSE
    /\ compacted = FALSE

PersistLocal ==
    /\ ~localDurable
    /\ ~compacted
    /\ localDurable' = TRUE
    /\ UNCHANGED <<clientAck, downstream, compacted>>

AckClient ==
    /\ localDurable
    /\ ~clientAck
    /\ clientAck' = TRUE
    /\ UNCHANGED <<localDurable, downstream, compacted>>

Deliver ==
    /\ localDurable
    /\ ~downstream
    /\ downstream' = TRUE
    /\ UNCHANGED <<localDurable, clientAck, compacted>>

Compact ==
    /\ downstream
    /\ localDurable
    /\ ~compacted
    /\ compacted' = TRUE
    /\ localDurable' = FALSE
    /\ UNCHANGED <<clientAck, downstream>>

Next == PersistLocal \/ AckClient \/ Deliver \/ Compact

Spec == Init /\ [][Next]_vars

TypeOK ==
    /\ localDurable \in BOOLEAN
    /\ clientAck \in BOOLEAN
    /\ downstream \in BOOLEAN
    /\ compacted \in BOOLEAN

AckHasDurableEvidence == clientAck => (localDurable \/ downstream)
NoEarlyCompaction == compacted => downstream
AckedUndeliveredStaysDurable == (clientAck /\ ~downstream) => localDurable

====
