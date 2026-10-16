output "queue" {
  description = "The partitioned queue the fulfilment consumers share"
  value       = solacebroker_msg_vpn_queue.orders.queue_name
}
