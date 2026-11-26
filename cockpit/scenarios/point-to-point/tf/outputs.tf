output "queue" {
  description = "The queue that takes the label jobs"
  value       = solacebroker_msg_vpn_queue.labels.queue_name
}
