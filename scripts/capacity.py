"""Per-pod capacity step test: highest /work rps one 1 CPU / 128 MiB container
serves with p95 <= SLO and <= 1% failures. Stdlib only."""
import argparse
import json
import os
import shutil
import subprocess
import tempfile
import time
import urllib.request
from datetime import datetime, timezone

IMAGE = "loadlab-backend:local"
SLO_MS = 200
MAX_FAILED_RATE = 0.01
STEP_RPS = 10
CEILING_RPS = 300  # safety cap so a misconfigured run can't step forever
BASE_URL = "http://localhost:3000"
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


def passes(level, slo_ms):
    return level["p95_ms"] <= slo_ms and level["failed_rate"] <= MAX_FAILED_RATE


def capacity_from_levels(levels, slo_ms):
    """rps of the last consecutive passing level before the first failure."""
    best = None
    for level in levels:
        if not passes(level, slo_ms):
            break
        best = level["rps"]
    if best is None:
        raise ValueError("first level already fails the SLO; no capacity measured")
    return best


def wait_ready(timeout_s=60):
    deadline = time.monotonic() + timeout_s
    while time.monotonic() < deadline:
        try:
            with urllib.request.urlopen(f"{BASE_URL}/readyz", timeout=2) as r:
                if r.status == 200:
                    return
        except OSError:
            pass
        time.sleep(0.5)
    raise RuntimeError(f"/readyz not ready after {timeout_s}s")


def run_step(k6, rps, duration_s, tmpdir):
    out = os.path.join(tmpdir, f"step-{rps}.json")
    subprocess.run(
        [k6, "run", "--quiet",
         "-e", f"RATE={rps}", "-e", f"DURATION={duration_s}s",
         "-e", f"BASE_URL={BASE_URL}", "-e", f"OUT={out}",
         os.path.join(ROOT, "k6", "capacity-test.js")],
        check=True,
    )
    with open(out) as f:
        s = json.load(f)
    return {"rps": rps, "p95_ms": round(s["p95_ms"], 2), "failed_rate": round(s["failed_rate"], 4)}


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--duration", type=int, default=60, help="seconds per step")
    args = ap.parse_args()

    docker, k6 = shutil.which("docker"), shutil.which("k6")
    if not docker or not k6:
        raise SystemExit("docker and k6 must be on PATH")

    cid = subprocess.run(
        [docker, "run", "-d", "--cpus", "1", "-m", "128m", "-p", "3000:3000", IMAGE],
        check=True, capture_output=True, text=True,
    ).stdout.strip()
    levels = []
    try:
        wait_ready()
        with tempfile.TemporaryDirectory() as tmpdir:
            for rps in range(STEP_RPS, CEILING_RPS + 1, STEP_RPS):
                level = run_step(k6, rps, args.duration, tmpdir)
                levels.append(level)
                print(f"rps={level['rps']} p95_ms={level['p95_ms']} failed_rate={level['failed_rate']}", flush=True)
                if not passes(level, SLO_MS):
                    break
            else:
                print(f"NOTE: reached {CEILING_RPS} rps ceiling without breaking the SLO")
    finally:
        subprocess.run([docker, "rm", "-f", cid], capture_output=True)

    rps_per_pod = capacity_from_levels(levels, SLO_MS)
    os.makedirs(os.path.join(ROOT, "results"), exist_ok=True)
    with open(os.path.join(ROOT, "results", "capacity.json"), "w") as f:
        json.dump({
            "rps_per_pod": rps_per_pod,
            "slo_ms": SLO_MS,
            "image": IMAGE,
            "levels": levels,
            "measured_at": datetime.now(timezone.utc).isoformat(timespec="seconds"),
            "step_duration_s": args.duration,
        }, f, indent=2)
        f.write("\n")
    print(f"rps_per_pod={rps_per_pod}")


if __name__ == "__main__":
    main()
