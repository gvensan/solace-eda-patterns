output "queues" {
  description = "The order queue and its dead message queue"
  value = [
    solacebroker_msg_vpn_queue.orders.queue_name,
    solacebroker_msg_vpn_queue.dmq.queue_name,
  ]
}
