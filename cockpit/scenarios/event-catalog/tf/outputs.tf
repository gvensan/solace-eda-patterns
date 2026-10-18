output "queues" {
  description = "The queues of this scenario"
  value       = [solacebroker_msg_vpn_queue.finance.queue_name]
}
