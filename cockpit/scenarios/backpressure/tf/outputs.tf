output "queues" {
  description = "The reporter's queue"
  value       = [solacebroker_msg_vpn_queue.readings.queue_name]
}
