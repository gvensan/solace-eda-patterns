output "queues" {
  description = "One queue per group"
  value       = [for q in solacebroker_msg_vpn_queue.group : q.queue_name]
}
