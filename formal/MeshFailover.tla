---- MODULE MeshFailover ----
EXTENDS FiniteSets, TLC

Nodes == {"edgeA", "edgeB", "edgeC"}

VARIABLES healthy, attempted, delivered, terminalFailure

vars == <<healthy, attempted, delivered, terminalFailure>>
Eligible == healthy \ attempted

Init ==
    /\ healthy = Nodes
    /\ attempted = {}
    /\ delivered = FALSE
    /\ terminalFailure = FALSE

ForwardSuccess(node) ==
    /\ node \in Eligible
    /\ ~delivered
    /\ ~terminalFailure
    /\ attempted' = attempted \cup {node}
    /\ delivered' = TRUE
    /\ UNCHANGED <<healthy, terminalFailure>>

ForwardFailure(node) ==
    /\ node \in Eligible
    /\ ~delivered
    /\ ~terminalFailure
    /\ attempted' = attempted \cup {node}
    /\ healthy' = healthy \ {node}
    /\ UNCHANGED <<delivered, terminalFailure>>

NoRoute ==
    /\ Eligible = {}
    /\ ~delivered
    /\ ~terminalFailure
    /\ terminalFailure' = TRUE
    /\ UNCHANGED <<healthy, attempted, delivered>>

Next ==
    (\E node \in Nodes: ForwardSuccess(node))
    \/ (\E node \in Nodes: ForwardFailure(node))
    \/ NoRoute

Spec == Init /\ [][Next]_vars

TypeOK ==
    /\ healthy \subseteq Nodes
    /\ attempted \subseteq Nodes
    /\ delivered \in BOOLEAN
    /\ terminalFailure \in BOOLEAN

NoSuccessAndTerminalFailure == ~(delivered /\ terminalFailure)
TerminalFailureHasNoRoute == terminalFailure => Eligible = {}
AttemptedOwnersAreBounded == attempted \subseteq Nodes

====
