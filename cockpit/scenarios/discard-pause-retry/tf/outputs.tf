output "queues" {
  description = "The queues of this scenario"
  value       = [solacebroker_msg_vpn_queue.dmq.queue_name, solacebroker_msg_vpn_queue.payments.queue_name, solacebroker_msg_vpn_queue.retry.queue_name]
}
