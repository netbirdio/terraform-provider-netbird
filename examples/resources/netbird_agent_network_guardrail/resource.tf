resource "netbird_agent_network_guardrail" "strict" {
  name        = "Strict — Production"
  description = "Tight model allowlist with PII redaction."

  model_allowlist = {
    enabled = true
    models  = ["gpt-4o-mini", "claude-haiku-4-5"]
  }

  prompt_capture = {
    enabled    = true
    redact_pii = true
  }
}
