# ---------------------------------------------------------------------------
# Event gateway: the way out, a REST delivery point
#
# The broker itself posts each message on q.gw.billing to billing's webhook
# over HTTP. Billing runs on the machine the broker's container runs on (or
# in the same Codespace), which the container reaches as
# host.docker.internal (setup_broker.sh adds that name). A message stays on
# the queue until the webhook answers 2xx, retried every few seconds.
# ---------------------------------------------------------------------------

variable "webhook_host" {
  description = "Where the broker reaches billing's webhook"
  type        = string
  default     = "host.docker.internal"
}

variable "webhook_port" {
  description = "Billing's webhook port; the dashboard passes SOLACE_WEBHOOK_PORT"
  type        = number
  default     = 18099
}

resource "solacebroker_msg_vpn_rest_delivery_point" "billing" {
  msg_vpn_name             = var.msg_vpn
  rest_delivery_point_name = "rdp-gw-billing"
  client_profile_name      = solacebroker_msg_vpn_client_profile.gw.client_profile_name
  enabled                  = true
}

resource "solacebroker_msg_vpn_rest_delivery_point_queue_binding" "billing" {
  msg_vpn_name             = var.msg_vpn
  rest_delivery_point_name = solacebroker_msg_vpn_rest_delivery_point.billing.rest_delivery_point_name
  queue_binding_name       = solacebroker_msg_vpn_queue.billing.queue_name
  post_request_target      = "/billing/orders"
}

resource "solacebroker_msg_vpn_rest_delivery_point_rest_consumer" "billing" {
  msg_vpn_name             = var.msg_vpn
  rest_delivery_point_name = solacebroker_msg_vpn_rest_delivery_point.billing.rest_delivery_point_name
  rest_consumer_name       = "billing-webhook"
  remote_host              = var.webhook_host
  remote_port              = var.webhook_port
  tls_enabled              = false
  retry_delay              = 3
  enabled                  = true
}
