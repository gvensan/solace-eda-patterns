output "queues" {
  description = "The queue of purchases"
  value       = [solacebroker_msg_vpn_queue.awards.queue_name]
}
