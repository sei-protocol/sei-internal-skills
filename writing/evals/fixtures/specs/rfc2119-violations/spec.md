<!-- Fixture: a specification. Each sentence below uses an RFC 2119 keyword in -->
<!-- lowercase, so each one MUST produce a finding. -->
<!-- vale AgenticWriting.Model-Tells = NO -->

The gateway should retry the request when the upstream returns a 503 status.
The client must not reconnect before the retry timer expires.
An implementation may cache the token.
A proxy is required to forward the header, and a retry is optional.
