#!/usr/bin/env python3
import argparse, subprocess
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
ap=argparse.ArgumentParser();ap.add_argument('--execute',action='store_true');ap.add_argument('--acknowledge-cluster-change',action='store_true');args=ap.parse_args()
if not args.execute or not args.acknowledge_cluster_change: raise SystemExit('requires --execute --acknowledge-cluster-change')
subprocess.run([str(ROOT/'proof/k8s-rollout.sh'),'--execute','--ack-cluster-change'],cwd=ROOT,check=True)
