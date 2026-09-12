---- MODULE CryptographicLineage ----
EXTENDS Naturals, TLC

VARIABLES count, chainOK, sealed, tampered, verified, rejected

vars == <<count, chainOK, sealed, tampered, verified, rejected>>
MaxRecords == 3

Init ==
    /\ count = 0
    /\ chainOK = TRUE
    /\ sealed = FALSE
    /\ tampered = FALSE
    /\ verified = FALSE
    /\ rejected = FALSE

AppendLinked ==
    /\ ~sealed
    /\ ~tampered
    /\ count < MaxRecords
    /\ count' = count + 1
    /\ UNCHANGED <<chainOK, sealed, tampered, verified, rejected>>

Seal ==
    /\ ~sealed
    /\ count > 0
    /\ chainOK
    /\ sealed' = TRUE
    /\ UNCHANGED <<count, chainOK, tampered, verified, rejected>>

VerifyClean ==
    /\ sealed
    /\ ~tampered
    /\ ~verified
    /\ ~rejected
    /\ verified' = TRUE
    /\ UNCHANGED <<count, chainOK, sealed, tampered, rejected>>

Tamper ==
    /\ sealed
    /\ ~tampered
    /\ ~verified
    /\ ~rejected
    /\ tampered' = TRUE
    /\ chainOK' = FALSE
    /\ UNCHANGED <<count, sealed, verified, rejected>>

RejectTampered ==
    /\ sealed
    /\ tampered
    /\ ~verified
    /\ ~rejected
    /\ rejected' = TRUE
    /\ UNCHANGED <<count, chainOK, sealed, tampered, verified>>

Next == AppendLinked \/ Seal \/ VerifyClean \/ Tamper \/ RejectTampered
Spec == Init /\ [][Next]_vars

TypeOK ==
    /\ count \in 0..MaxRecords
    /\ chainOK \in BOOLEAN
    /\ sealed \in BOOLEAN
    /\ tampered \in BOOLEAN
    /\ verified \in BOOLEAN
    /\ rejected \in BOOLEAN

VerifiedHistoryIsUntampered == verified => ~tampered
TamperedHistoryCannotVerify == tampered => ~verified
RejectedHistoryWasTampered == rejected => tampered
SealHasRecords == sealed => count > 0

====
