"""Benchmark isolated local gateways against the services selected by the environment."""

import argparse
import json
import socket
import subprocess
import tempfile
import time
import urllib.request
from pathlib import Path


def endpoint(port, path):
    with urllib.request.urlopen(
        f"http://127.0.0.1:{port}/{path}", timeout=3
    ) as response:
        return response.read()


def processes(pids):
    result = subprocess.run(
        ["ps", "-o", "pid=,rss=,time=", "-p", ",".join(map(str, pids))],
        capture_output=True,
        text=True,
        check=False,
    )
    samples = {}
    for line in result.stdout.splitlines():
        pid, rss, elapsed = line.split()
        cpu = 0.0
        for part in elapsed.split(":"):
            cpu = cpu * 60 + float(part)
        samples[int(pid)] = (int(rss), cpu)
    return samples


def run(args, gateway, client, ports, window, gap):
    children = []
    load = None
    try:
        for port in ports:
            with socket.socket() as check:
                check.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
                check.bind(("127.0.0.1", port))
            child = subprocess.Popen(
                [
                    gateway,
                    "-port",
                    str(port),
                    "-batch-window",
                    window,
                    "-batch-arrival-gap",
                    gap,
                ],
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )
            children.append(child)
            for _ in range(100):
                if child.poll() is not None:
                    raise RuntimeError(f"gateway exited on port {port}")
                try:
                    endpoint(port, "readyz")
                    break
                except OSError:
                    time.sleep(0.1)
            else:
                raise RuntimeError(f"gateway {port} not ready")
        addresses = [f"127.0.0.1:{p}" for p in ports]
        cmd = [
            client,
            "-addr",
            addresses[0],
            "-browser",
            "-viewers",
            str(args.viewers),
            "-rate",
            str(args.rate),
            "-seconds",
            str(args.seconds),
            "-drain",
            str(args.drain) + "s",
        ]
        if len(ports) > 1:
            cmd.extend(["-peers", ",".join(addresses[1:])])
        ids = [p.pid for p in children]
        base = processes(ids)
        peak = client_peak = 0
        client_cpu = 0.0
        timeline = []
        start = time.monotonic()
        next_sample = 30
        with tempfile.TemporaryFile(mode="w+") as log:
            load = subprocess.Popen(cmd, stdout=log, stderr=subprocess.STDOUT)
            while load.poll() is None:
                sample = processes(ids + [load.pid])
                peak = max(peak, sum(sample.get(pid, (0, 0))[0] for pid in ids))
                client_peak = max(client_peak, sample.get(load.pid, (0, 0))[0])
                client_cpu = max(client_cpu, sample.get(load.pid, (0, 0))[1])
                elapsed = time.monotonic() - start
                if args.seconds >= 60 and elapsed >= next_sample:
                    timeline.append(
                        {
                            "elapsed_s": round(elapsed),
                            "gateway_rss_kib": sum(
                                sample.get(pid, (0, 0))[0] for pid in ids
                            ),
                            "client_rss_kib": sample.get(load.pid, (0, 0))[0],
                            "metrics": [
                                json.loads(endpoint(port, "metrics")) for port in ports
                            ],
                        }
                    )
                    next_sample += 30
                time.sleep(0.5)
            log.seek(0)
            output = log.read().splitlines()
        final = processes(ids)
        return {
            "viewers": args.viewers,
            "rate": args.rate,
            "seconds": args.seconds,
            "gateways": len(ports),
            "batch_window": window,
            "batch_arrival_gap": gap,
            "exit": load.returncode,
            "gateway_cpu_s": round(sum(final[i][1] - base[i][1] for i in ids), 2),
            "gateway_peak_rss_kib": peak,
            "client_peak_rss_kib": client_peak,
            "client_sampled_cpu_s": client_cpu,
            "timeline": timeline,
            "output": output,
            "metrics": [json.loads(endpoint(port, "metrics")) for port in ports],
        }
    finally:
        if load is not None and load.poll() is None:
            load.terminate()
            load.wait()
        for child in children:
            if child.poll() is None:
                child.terminate()
        for child in children:
            child.wait()


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--viewers", type=int, default=1000)
    p.add_argument("--gateways", type=int, default=2)
    p.add_argument("--rate", type=int, default=25)
    p.add_argument("--seconds", type=int, default=10)
    p.add_argument("--drain", type=int, default=10)
    p.add_argument("--rounds", type=int, default=1)
    p.add_argument("--port", type=int, default=38801)
    p.add_argument(
        "--policies", default="5ms/2ms", help="comma-separated window/arrival-gap pairs"
    )
    args = p.parse_args()
    if min(args.viewers, args.gateways, args.rate, args.seconds, args.rounds) < 1:
        p.error("expected positive workload parameters")
    with tempfile.TemporaryDirectory() as tmp:
        gateway, client = [str(Path(tmp) / n) for n in ("gateway", "client")]
        for out, pkg in [(gateway, "./cmd/server"), (client, "./cmd/benchmark/fanout")]:
            subprocess.run(["go", "build", "-o", out, pkg], check=True)
        policies = [item.split("/") for item in args.policies.split(",")]
        failed = False
        for i in range(args.rounds):
            for window, gap in (
                policies[i % len(policies) :] + policies[: i % len(policies)]
            ):
                result = run(
                    args,
                    gateway,
                    client,
                    range(args.port, args.port + args.gateways),
                    window,
                    gap,
                )
                print(json.dumps(result), flush=True)
                failed |= result["exit"] != 0
        return int(failed)


if __name__ == "__main__":
    raise SystemExit(main())
