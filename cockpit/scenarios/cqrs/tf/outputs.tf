output "queues" {
  description = "The queues of this scenario"
  value       = [solacebroker_msg_vpn_queue.commands.queue_name, solacebroker_msg_vpn_queue.view.queue_name]
}
