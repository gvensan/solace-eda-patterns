output "queue" {
  description = "The queue every worker shares"
  value       = solacebroker_msg_vpn_queue.emails.queue_name
}
