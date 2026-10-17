# ---------------------------------------------------------------------------
# Event bridge: the bridge
#
# A bridge in the stores' VPN that logs in to head office's VPN and consumes
# q.br.to-stores, publishing what it takes into the stores' VPN on the same
# topics. Head office's VPN is on this broker, so the remote address is the
# broker's own messaging port inside its container; to another broker it
# would be that broker's address. Guaranteed messages wait on
# q.br.to-stores while the bridge is down (docs/spikes.md, section 19).
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_bridge" "hq" {
  msg_vpn_name          = var.msg_vpn
  bridge_name           = "br-hq-link"
  bridge_virtual_router = "auto"
  enabled               = true

  remote_authentication_scheme                = "basic"
  remote_authentication_basic_client_username = solacebroker_msg_vpn_client_username.hq["bridge"].client_username
  remote_authentication_basic_password        = var.client_password
}

resource "solacebroker_msg_vpn_bridge_remote_msg_vpn" "hq" {
  msg_vpn_name             = var.msg_vpn
  bridge_name              = solacebroker_msg_vpn_bridge.hq.bridge_name
  bridge_virtual_router    = "auto"
  remote_msg_vpn_name      = solacebroker_msg_vpn.hq.msg_vpn_name
  remote_msg_vpn_location  = "127.0.0.1:55555"
  remote_msg_vpn_interface = ""
  queue_binding            = solacebroker_msg_vpn_queue.to_stores.queue_name
  enabled                  = true

  depends_on = [solacebroker_msg_vpn_queue_subscription.to_stores]
}
