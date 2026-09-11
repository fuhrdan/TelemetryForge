#!/usr/bin/env python3
import argparse, subprocess
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
ap=argparse.ArgumentParser();ap.add_argument('--execute',action='store_true');args=ap.parse_args()
if not args.execute: raise SystemExit('refusing backup/restore without --execute')
subprocess.run([str(ROOT/'proof/backup-restore.sh'),'--execute'],cwd=ROOT,check=True)
