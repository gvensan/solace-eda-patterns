output "queues" {
  description = "The queues of this scenario"
  value       = [solacebroker_msg_vpn_queue.berlin.queue_name, solacebroker_msg_vpn_queue.alarms.queue_name]
}
