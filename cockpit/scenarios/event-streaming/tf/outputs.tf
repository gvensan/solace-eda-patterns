output "queues" {
  description = "The stream, kept in order for the counter"
  value       = [solacebroker_msg_vpn_queue.rides.queue_name]
}
