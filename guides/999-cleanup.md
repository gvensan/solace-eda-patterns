# Cleanup

Nothing here is required if you are throwing the container away, which in a
Codespace you usually are. It matters if you are sharing a broker or coming
back to a clean slate.

## Per scenario

Each scenario's **Cleanup** button stops its apps and removes its broker
configuration. It asks first, because it deletes queues and anything still
spooled on them.

Scenarios do not share configuration, so cleaning one up never affects
another.

## Everything at once

**Stop everything** in the top bar ends every running process across all
scenarios without touching broker configuration. Reach for it when you have
lost track of what is running.

**Clear broker config**, next to it, stops everything and then deletes every
queue, profile and username that any scenario creates, along with every
scenario's terraform state. Broker defaults and anything you made by hand are
left alone.

**Reset this scenario**, under Step through it, stops that scenario's
processes and deletes its terraform state. It does **not** remove what is
already on the broker, so run the scenario's Cleanup first if you want the
broker clean. Resetting without cleaning up is the usual way to end up
needing **Reconcile with broker** later.

## Starting over completely

```bash
docker rm -f solace_10.26.0
bash setup_broker.sh
rm -rf cockpit/.state
```

That is a new broker with nothing on it and no terraform state. The next Play
in any scenario creates everything from scratch, including rebuilding the Go
binary.
