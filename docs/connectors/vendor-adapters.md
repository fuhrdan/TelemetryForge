# Vendor HTTP Adapters

v1.8 includes small HTTP adapters for Splunk HEC and Datadog Logs without importing vendor SDKs. Splunk uses an environment-backed HEC token; Datadog uses an environment-backed API key. Health probes use the same vendor authentication as delivery when a health endpoint is configured.

The checked-in examples are disabled and contain only environment variable names, never credentials. More advanced vendor capabilities can be added later behind the same connector contract.
