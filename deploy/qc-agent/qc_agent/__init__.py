"""QC bridge home agent (plan P4): runs signed device jobs from the jobs bucket on
the owner's Quad Cortex. See deploy/qc-agent/README.md."""

from .version import agent_version

# "<release>+<commit of the checkout this process started from>": see version.py
AGENT_VERSION = agent_version()
