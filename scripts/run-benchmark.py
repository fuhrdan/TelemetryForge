#!/usr/bin/env python3
from __future__ import annotations
import argparse, subprocess
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
def main():
 ap=argparse.ArgumentParser();ap.add_argument('scenario',choices=['smoke','sustained','backpressure']);ap.add_argument('--execute',action='store_true');args=ap.parse_args()
 if not args.execute: raise SystemExit('refusing benchmark execution without --execute')
 subprocess.run([str(ROOT/'proof/run-k6.sh'),args.scenario,'--execute'],cwd=ROOT,check=True)
if __name__=='__main__':main()
