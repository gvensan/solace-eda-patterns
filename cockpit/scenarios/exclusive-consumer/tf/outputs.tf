output "queue" {
  description = "The exclusive queue both ledger instances bind"
  value       = solacebroker_msg_vpn_queue.postings.queue_name
}
