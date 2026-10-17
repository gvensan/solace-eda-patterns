output "queues" {
  description = "The absorber and its dead message queue"
  value       = [solacebroker_msg_vpn_queue.purchases.queue_name, solacebroker_msg_vpn_queue.dmq.queue_name]
}
