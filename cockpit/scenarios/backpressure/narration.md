# Narration: Backpressure and Push
# Spoken by the page's audio player. Lines starting with # are not spoken.
# After editing, run: python3 cockpit/scripts/narrate.py backpressure

Backpressure is how a consumer that cannot keep up slows the flow to itself, instead of being flooded or losing work. A broker pushes messages to consumers without being asked, so a consumer needs a way to say: not so fast. And whatever it cannot take has to wait somewhere, or be dropped.

Cold-chain monitoring shows both needs. Sensors in a refrigerated warehouse report forty readings a second. A reporter writes every reading to a compliance database, which slows down at times. A live display on the wall only cares about the latest temperatures.

Here, the reporter reads a queue, and acknowledges each reading only once it is written. The broker stops sending once the reporter holds a hundred unacknowledged readings, and the rest wait on the queue. The display subscribes directly, with no queue. When it falls behind, its messaging API drops readings from a small buffer.

The price is that a slow consumer still falls behind. Backpressure only decides where the backlog waits, or what is thrown away.

The Break it cards show both choices. Slow the database down, and watch the reporter pause, the broker hold back, and the readings wait on the queue, then arrive. Slow the display down, and watch it drop the oldest readings, so what it shows stays fresh.

Press Play, and watch the reporter and the display take the same readings in two ways.
