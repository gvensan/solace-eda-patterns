# ---------------------------------------------------------------------------
# One queue per group
#
# Solace has no consumer group object. A group is a queue: every group's
# queue subscribes to the same topic, so each stores its own copy of every
# price change and keeps its own backlog. The group's members share the
# queue. Partitioned by product, so a group sees each product's changes in
# order; two partitions, for the two members a group is meant to run.
# ---------------------------------------------------------------------------

locals {
  cg_groups = ["search", "alerts"]
}

resource "solacebroker_msg_vpn_queue" "group" {
  for_each = toset(local.cg_groups)

  msg_vpn_name = var.msg_vpn
  queue_name   = "q.cg.${each.key}"

  access_type     = "non-exclusive"
  ingress_enabled = true
  egress_enabled  = true
  permission      = "consume"

  partition_count                      = 2
  partition_rebalance_delay            = 5
  partition_rebalance_max_handoff_time = 3

  max_msg_spool_usage                 = 50
  max_delivered_unacked_msgs_per_flow = 10
}

resource "solacebroker_msg_vpn_queue_subscription" "group" {
  for_each = toset(local.cg_groups)

  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.group[each.key].queue_name
  subscription_topic = "cg/catalog/price/changed/v1/>"
}
