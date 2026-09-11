#!/usr/bin/env python3
from __future__ import annotations
import argparse, subprocess, sys
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
MAP={
 'kafka-outage':['proof/kafka-outage.sh'],
 'database-outage':['proof/postgres-outage.sh'],
 'postgres-outage':['proof/postgres-outage.sh'],
 'connector-outage':['proof/connector-outage.sh'],
 'worker-failover':['proof/worker-failover.sh'],
 'preflight':[],
}
def main():
 ap=argparse.ArgumentParser();ap.add_argument('scenario',choices=MAP);ap.add_argument('--execute',action='store_true');args=ap.parse_args()
 if args.scenario=='preflight':
  cmds=[['docker','compose','config','--quiet'],['python3','scripts/check-docs.py']]
  for cmd in cmds: subprocess.run(cmd,cwd=ROOT,check=True)
  print('preflight passed'); return
 if not args.execute: raise SystemExit('refusing disruptive scenario without --execute')
 cmd=[str(ROOT/MAP[args.scenario][0]),'--execute']; subprocess.run(cmd,cwd=ROOT,check=True)
if __name__=='__main__':main()
