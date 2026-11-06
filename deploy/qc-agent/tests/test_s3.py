"""The stdlib S3 client: AWS's published SigV4 examples, bounded reads, pagination, no redirects."""
import datetime as dt
import io
import unittest
import urllib.error

import helpers  # noqa: F401  (puts qc_agent on sys.path)
from qc_agent import s3

AK, SK = "AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
DAY = dt.datetime(2013, 5, 24, tzinfo=dt.timezone.utc)


class SigV4(unittest.TestCase):
    """Examples from AWS's "Signature Version 4 (header) examples" for S3."""

    def test_get_object_with_range(self):
        h = s3.sign_v4("GET", "examplebucket.s3.amazonaws.com", "/test.txt", {}, {"range": "bytes=0-9"},
                       s3.EMPTY_SHA256, AK, SK, "us-east-1", "s3", DAY)
        self.assertTrue(h["authorization"].endswith(
            "Signature=f0e8bdb87c964420e857bd35b5d6ed310bd44f0170aba48dd91039c6036bdb41"))

    def test_list_objects(self):
        h = s3.sign_v4("GET", "examplebucket.s3.amazonaws.com", "/", {"max-keys": "2", "prefix": "J"}, {},
                       s3.EMPTY_SHA256, AK, SK, "us-east-1", "s3", DAY)
        self.assertTrue(h["authorization"].endswith(
            "Signature=34b48302e7b5fa45bde8084f4b7868a86f0a534bc59db6670ed5711ef69dc6f7"))


class Resp(io.BytesIO):
    def __init__(self, status, body):
        super().__init__(body)
        self.status = status

    def __enter__(self):
        return self

    def __exit__(self, *a):
        pass


class Opener:
    def __init__(self, responses):
        self.responses = list(responses)
        self.requests = []

    def __call__(self, req, timeout=None):
        self.requests.append(req)
        status, body = self.responses.pop(0)
        if status >= 300:
            raise urllib.error.HTTPError(req.full_url, status, "x", {}, io.BytesIO(body))
        return Resp(status, body)


def listing(keys, token=None):
    ns = 'xmlns="http://s3.amazonaws.com/doc/2006-03-01/"'
    items = "".join(f"<Contents><Key>{k}</Key><Size>{n}</Size></Contents>" for k, n in keys)
    more = f"<IsTruncated>true</IsTruncated><NextContinuationToken>{token}</NextContinuationToken>" if token \
        else "<IsTruncated>false</IsTruncated>"
    return f'<?xml version="1.0"?><ListBucketResult {ns}>{items}{more}</ListBucketResult>'.encode()


def store(responses):
    op = Opener(responses)
    return s3.S3Store("https://acct.r2.cloudflarestorage.com", "jobs-bucket", AK, SK, opener=op), op


class Store(unittest.TestCase):
    def test_list_paginates(self):
        st, op = store([(200, listing([("jobs/pending/a.json", 10)], token="t1")),
                        (200, listing([("jobs/pending/b.json", 20)]))])
        items, more = st.list_prefix("jobs/pending/", 50)
        self.assertEqual(items, [("jobs/pending/a.json", 10), ("jobs/pending/b.json", 20)])
        self.assertFalse(more)
        self.assertIn("continuation-token=t1", op.requests[1].full_url)
        self.assertTrue(op.requests[0].full_url.startswith("https://acct.r2.cloudflarestorage.com/jobs-bucket?"))

    def test_list_caps(self):
        st, _ = store([(200, listing([(f"k{i}", 1) for i in range(5)]))])
        items, more = st.list_prefix("", 3)
        self.assertEqual(len(items), 3)
        self.assertTrue(more)

    def test_list_rejects_dtd(self):
        st, _ = store([(200, b'<?xml version="1.0"?><!DOCTYPE x [<!ENTITY a "b">]><x/>')])
        with self.assertRaisesRegex(s3.StoreError, "unexpected DTD"):
            st.list_prefix("", 3)

    def test_bounded_get(self):
        st, op = store([(206, b"x" * 11)])
        self.assertIsNone(st.get_bounded("jobs/pending/a.json", 10))
        self.assertEqual(op.requests[0].get_header("Range"), "bytes=0-10")
        st, _ = store([(206, b"x" * 10)])
        self.assertEqual(st.get_bounded("k", 10), b"x" * 10)
        st, _ = store([(416, b"")])
        self.assertEqual(st.get_bounded("k", 10), b"")

    def test_head_put_delete(self):
        st, op = store([(200, b""), (404, b""), (200, b""), (204, b""), (404, b"")])
        self.assertTrue(st.exists("a"))
        self.assertFalse(st.exists("b"))
        st.put("jobs/done/x.json", b"{}")
        st.delete("x")
        st.delete("gone")
        self.assertEqual(op.requests[2].get_method(), "PUT")
        self.assertEqual(op.requests[2].data, b"{}")

    def test_errors_carry_no_credentials(self):
        st, _ = store([(403, b"<Error><Code>AccessDenied</Code></Error>")])
        with self.assertRaisesRegex(s3.StoreError, "AccessDenied") as cm:
            st.put("k", b"{}")
        self.assertNotIn(SK, str(cm.exception))
        self.assertNotIn(SK, repr(st))

    def test_truncated_response_is_a_store_error(self):
        import http.client

        def cut(req, timeout=None):
            raise http.client.IncompleteRead(b"partial")
        st = s3.S3Store("https://acct.r2.cloudflarestorage.com", "jobs-bucket", AK, SK, opener=cut)
        with self.assertRaisesRegex(s3.StoreError, "IncompleteRead"):
            st.get_bounded("k", 10)

    def test_redirects_are_not_followed(self):
        h = s3._NoRedirect()
        self.assertIsNone(h.redirect_request(None, None, 301, "x", {}, "https://elsewhere/"))

    def test_key_encoding(self):
        st, op = store([(200, b"")])
        st.exists("jobs/done/a b.json")
        self.assertTrue(op.requests[0].full_url.endswith("/jobs-bucket/jobs/done/a%20b.json"))


if __name__ == "__main__":
    unittest.main()
