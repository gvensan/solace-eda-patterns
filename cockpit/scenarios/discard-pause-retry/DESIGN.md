# Discard, Pause and Retry (620): design

Category 6, Error Handling Patterns, second pattern. Prefix `dpr`.

**One idea.** When processing fails, decide by what failed: a message that
can never succeed is discarded (dead message queue), a message that failed
because of the moment is retried later, and a broken dependency pauses
consumption altogether.

**Story.** A payment processor charges each payment at the bank by
request-reply (`dpr/bank/charge/v1/<acct>`, 1 s timeout).

**Broker objects.** `q.dpr.payments` (`dpr/pay/payment/requested/v1/>`),
`q.dpr.retry` (`dpr/pay/payment/retry/v1/>`, `delivery_delay` 5 s), both
with dead message queue `q.dpr.dmq`. `cp-dpr`, `acl-dpr`, usernames
`svc-dpr-shop`, `-bank`, `-processor`. The broker's delayed redelivery is
not usable from Go (spikes section 5), so the retry is the app republishing
to a delayed queue, the design section 5 proposed; verified here end to end.

**Apps.** `dpr payments` (`--bad` sends an amount that is not a number),
`dpr bank` (`--busy-every N`), `dpr processor` (binds both queues; REJECTED
to discard; republish with attempt+1 to retry, max 3; on no answer, Pause
both receivers, probe every 2 s, Resume).

**Play.** apply, build, bank, processor, shop.

**Cards.** The bank goes down (pause); malformed payments arrive (discard);
the bank is busy now and then (retry after 5 s).
