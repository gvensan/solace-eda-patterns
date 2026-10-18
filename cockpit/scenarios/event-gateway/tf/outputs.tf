output "queues" {
  description = "The queues of this scenario"
  value       = [solacebroker_msg_vpn_queue.dispatch.queue_name, solacebroker_msg_vpn_queue.billing.queue_name]
}
