"""Every entry of testdata/devicejobs/vectors.json, through the agent's code.

Three passes:
* the agent's contract module (its copy of the Python reference) reaches every `expect`;
* the original reference reaches the same outcomes (so the copy hasn't drifted);
* each pending_jobs vector goes through Agent._check against an in-memory bucket, and the
  agent takes exactly the action contract section 8 names for that outcome.
"""
import datetime as dt
import importlib.util
import json
import tempfile
import unittest
from pathlib import Path

from helpers import VECTORS, REPO, catalog, config
from qc_agent import contract
from qc_agent.agent import Agent
from qc_agent.s3 import MemoryStore
from fake_device import FakeDevice

V = json.loads(VECTORS.read_text(encoding="utf-8"))


def ring(name):
    r = V["keyrings"][name]
    return contract.keyring(V["keys"][r["current"]], V["keys"].get(r.get("next", "")))


def load_reference():
    spec = importlib.util.spec_from_file_location("reference_check", REPO / "testdata/devicejobs/reference_check.py")
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


class VectorCounts(unittest.TestCase):
    def test_vector_file_has_entries(self):
        self.assertGreaterEqual(len(V["signatures"]) + len(V["envelopes"]) + len(V["pending_jobs"]), 79)


class Signatures(unittest.TestCase):
    def test_signatures(self):
        for s in V["signatures"]:
            with self.subTest(s["name"]):
                key = contract.parse_key(V["keys"][s["key"] + "_hex"])
                self.assertEqual(contract.sign(key, s["tag"], s["payload"]), s["sig"])


class Envelopes(unittest.TestCase):
    def test_envelopes(self):
        for e in V["envelopes"]:
            with self.subTest(e["name"]):
                try:
                    payload, got = contract.open_envelope(ring(e["keyring"]), e["tag"], e["envelope"].encode()), "ok"
                except contract.Reject as r:
                    payload, got = None, r.outcome
                self.assertEqual(got, e["expect"])
                if got == "ok":
                    self.assertEqual(payload, e["payload"])


def outcome(mod, c):
    ledger = set(c["ledger"])
    try:
        mod.check_pending_job(ring(c["keyring"]) if mod is contract else
                              mod.keyring(V["keys"][V["keyrings"][c["keyring"]]["current"]],
                                          V["keys"].get(V["keyrings"][c["keyring"]].get("next", ""))),
                              c["object_key"], c["agent_bucket"], mod.parse_time(c["now"]),
                              c["envelope"].encode(), lambda i: i in ledger)
        return "ok"
    except mod.Reject as r:
        return r.outcome


class PendingJobs(unittest.TestCase):
    def test_agent_contract_module(self):
        for c in V["pending_jobs"]:
            with self.subTest(c["name"]):
                self.assertEqual(outcome(contract, c), c["expect"])

    def test_reference_agrees(self):
        ref = load_reference()
        for c in V["pending_jobs"]:
            with self.subTest(c["name"]):
                self.assertEqual(outcome(ref, c), outcome(contract, c))


class FixedClock:
    def __init__(self, t):
        self.t = t

    def __call__(self):
        return self.t


class AgentActions(unittest.TestCase):
    """The agent's action for each outcome (contract section 8 table)."""

    def run_vector(self, c):
        r = V["keyrings"][c["keyring"]]
        over = {"DEVICE_JOB_KEY": V["keys"][r["current"]], "DEVICE_JOB_KEY_NEXT": V["keys"].get(r.get("next", "")),
                "QC_AGENT_R2_BUCKET": c["agent_bucket"]}
        cfg = config(**over)
        store = MemoryStore(c["agent_bucket"])
        key = c["object_key"]
        store.objects[key] = c["envelope"].encode()
        with tempfile.TemporaryDirectory() as d:
            now = contract.parse_time(c["now"])
            agent = Agent(cfg, store, FakeDevice(), catalog(), Path(d), now_fn=FixedClock(now))
            for jid in c["ledger"]:
                agent.ledger.take(jid, "list_slots", c["now"])
                agent.ledger.finish(jid, c["now"], b'{"stub":1}')
            got = agent._check(key, store.objects[key])
            return got, store, agent

    def test_every_vector_action(self):
        for c in V["pending_jobs"]:
            with self.subTest(c["name"]):
                got, store, agent = self.run_vector(c)
                key = c["object_key"]
                done = [k for k in store.objects if k.startswith("jobs/done/") and k != key]
                exp = c["expect"]
                if exp == "ok":
                    self.assertIsNotNone(got)
                    self.assertIn(key, store.objects)
                    self.assertEqual(done, [])
                elif exp == "bad_object_key":
                    self.assertIsNone(got)
                    self.assertIn(key, store.objects)            # skipped, not deleted
                    self.assertEqual(done, [])
                elif exp == "expired":
                    self.assertIsNone(got)
                    self.assertNotIn(key, store.objects)
                    self.assertEqual(len(done), 1)
                    res = json.loads(contract.open_envelope(agent.config.keys, contract.TAG_RESULT,
                                                            store.objects[done[0]]))
                    self.assertEqual((res["status"], res["reason_code"]), ("expired", "expired"))
                    self.assertTrue(agent.ledger.contains(res["job_id"]))
                else:                                            # forgeries, bad payloads, misrouted, replay
                    self.assertIsNone(got)
                    self.assertNotIn(key, store.objects)
                    self.assertEqual(done, [], "the agent never answers a forgery or a replay")


if __name__ == "__main__":
    unittest.main()
