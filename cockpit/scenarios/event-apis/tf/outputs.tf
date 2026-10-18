output "queues" {
  description = "The queues of this scenario"
  value       = [solacebroker_msg_vpn_queue.labels.queue_name, solacebroker_msg_vpn_queue.tracking.queue_name]
}
