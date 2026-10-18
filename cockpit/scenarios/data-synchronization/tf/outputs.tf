output "queues" {
  description = "The queues of this scenario"
  value       = [solacebroker_msg_vpn_queue.legacy.queue_name, solacebroker_msg_vpn_queue.new.queue_name]
}
