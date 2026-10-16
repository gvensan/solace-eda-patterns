output "queue" {
  description = "The queue the order service addresses by name"
  value       = solacebroker_msg_vpn_queue.labels.queue_name
}
