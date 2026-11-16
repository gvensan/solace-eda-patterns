# ---------------------------------------------------------------------------
# Event sourcing: the replay log
#
# A message VPN has at most one replay log, so this scenario owns the VPN's
# (docs/spikes.md, section 17). The topic filter keeps it to this
# scenario's events and the ledger's end-of-replay markers, not the
# commands: without it, every guaranteed message published in the VPN would
# be logged. A marker sent during a replay reaches the queue under replay
# only through the log, after the replayed events (section 23).
#
# max_spool_usage is in MB. 1 MB is the smallest the broker accepts, and it
# is deliberately small: when the log is full, the oldest messages are
# dropped without a word. A replay log is bounded and is not a store of
# record.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_replay_log" "esrc" {
  msg_vpn_name    = var.msg_vpn
  replay_log_name = "rl-esrc"

  max_spool_usage      = 1
  ingress_enabled      = true
  egress_enabled       = true
  topic_filter_enabled = true
}

resource "solacebroker_msg_vpn_replay_log_topic_filter_subscription" "esrc" {
  msg_vpn_name              = var.msg_vpn
  replay_log_name           = solacebroker_msg_vpn_replay_log.esrc.replay_log_name
  topic_filter_subscription = "esrc/bank/account/>"
}

resource "solacebroker_msg_vpn_replay_log_topic_filter_subscription" "esrc_marker" {
  msg_vpn_name              = var.msg_vpn
  replay_log_name           = solacebroker_msg_vpn_replay_log.esrc.replay_log_name
  topic_filter_subscription = "esrc/bank/marker/>"
}
