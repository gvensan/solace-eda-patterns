# ---------------------------------------------------------------------------
# Discard, pause and retry: queues
#
# q.dpr.payments holds new payments. q.dpr.retry holds payments the bank
# was too busy for: its delivery delay keeps each one back for 5 seconds
# before the processor can have it, which is the retry's wait. The broker's
# own delayed redelivery needs local transactions, which the Go API does not
# have (docs/spikes.md, section 5). A payment the processor rejects goes to
# q.dpr.dmq, the dead message queue of both.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_queue" "dmq" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.dpr.dmq"

  access_type                         = "exclusive"
  ingress_enabled                     = true
  egress_enabled                      = true
  permission                          = "consume"
  max_msg_spool_usage                 = 50
  max_delivered_unacked_msgs_per_flow = 10
}

resource "solacebroker_msg_vpn_queue" "payments" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.dpr.payments"

  access_type                         = "exclusive"
  ingress_enabled                     = true
  egress_enabled                      = true
  permission                          = "consume"
  max_msg_spool_usage                 = 50
  max_delivered_unacked_msgs_per_flow = 10
  dead_msg_queue                      = solacebroker_msg_vpn_queue.dmq.queue_name
}

resource "solacebroker_msg_vpn_queue_subscription" "payments" {
  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.payments.queue_name
  subscription_topic = "dpr/pay/payment/requested/v1/>"
}

resource "solacebroker_msg_vpn_queue" "retry" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.dpr.retry"

  access_type                         = "exclusive"
  ingress_enabled                     = true
  egress_enabled                      = true
  permission                          = "consume"
  max_msg_spool_usage                 = 50
  max_delivered_unacked_msgs_per_flow = 10
  delivery_delay                      = 5
  dead_msg_queue                      = solacebroker_msg_vpn_queue.dmq.queue_name
}

resource "solacebroker_msg_vpn_queue_subscription" "retry" {
  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.retry.queue_name
  subscription_topic = "dpr/pay/payment/retry/v1/>"
}
