# Narration: Event Filtering
# Spoken by the page's audio player. Lines starting with # are not spoken.
# After editing, run: python3 cockpit/scripts/narrate.py event-filtering

Event filtering means each consumer receives only the events it wants, and the filtering happens before delivery, not inside the consumer. On Solace, the first filter is the topic: a subscription says which events to deliver, and the broker sends nothing else. For a fact that is not in the topic, a selector can filter on a message property instead.

A factory is a natural example. Sensors on every line, at two sites, report readings several times a second. One desk wants Berlin's temperatures. Another wants every reading with severity four or five, from anywhere.

Here, the Berlin desk's queue subscribes to Berlin temperature topics, and the broker never puts anything else on it. Severity is not in the topic, so the alarm desk's queue takes every reading, and its consumer binds with the selector: severity at least four. The broker delivers only the matches.

The price is what a selector leaves behind. Readings it does not match stay on the queue, and nobody will ever take them, so the queue only grows.

The Break it cards show where that leads. The sensors switch to high resolution, the alarm queue fills its quota, and the broker starts refusing readings, for every subscriber, not just the alarm desk. Then a typo in the selector stops the alarm desk from binding at all.

Press Play, and watch the number left on the alarm queue.
