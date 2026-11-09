// Keeps one chat tab per browser. splash keeps one Ergo connection per session, so a second tab
// closes the first tab's connection. This causes both sessions to contend for the connection.
//
// This script runs on each new tab before gamja, disconnecting older tabs and avoiding contention.
(() => {
	if (!("BroadcastChannel" in window)) {
		return;
	}
	const channel = new BroadcastChannel("irc-tab");
	channel.onmessage = (event) => {
		if (event.data === "open") {
			channel.close();
			location.replace("/disconnected");
		}
	};
	channel.postMessage("open");
})();
