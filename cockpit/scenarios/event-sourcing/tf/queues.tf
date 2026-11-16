# ---------------------------------------------------------------------------
# Event sourcing: queues
#
# q.esrc.commands holds the teller's commands until the ledger takes them,
# so none is lost while the ledger is down or rebuilding. Exclusive: one
# ledger at a time takes commands, and a second would be a standby.
#
# q.esrc.rebuild, below, is where the history is replayed when the ledger
# starts. It holds events only during a rebuild.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_queue" "commands" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.esrc.commands"

  access_type                         = "exclusive"
  ingress_enabled                     = true
  egress_enabled                      = true
  permission                          = "consume"
  max_msg_spool_usage                 = 50
  max_delivered_unacked_msgs_per_flow = 10
}

resource "solacebroker_msg_vpn_queue_subscription" "commands" {
  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.commands.queue_name
  subscription_topic = "esrc/bank/command/>"
}

# q.esrc.rebuild is where the history is replayed at start: the broker
# replays only into a queue that subscribes to the events. The ledger owns
# it and adds that subscription only for the rebuild, then removes it, so no
# live event lands here. The marker subscription is the ledger's own
# end-of-replay message.
resource "solacebroker_msg_vpn_queue" "rebuild" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.esrc.rebuild"

  access_type                              = "exclusive"
  ingress_enabled                          = true
  egress_enabled                           = true
  owner                                    = "svc-esrc-ledger"
  permission                               = "no-access"
  max_msg_spool_usage                      = 10
  reject_msg_to_sender_on_discard_behavior = "never"

  depends_on = [solacebroker_msg_vpn_client_username.esrc]
}

resource "solacebroker_msg_vpn_queue_subscription" "rebuild_marker" {
  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.rebuild.queue_name
  subscription_topic = "esrc/bank/marker/>"
}
