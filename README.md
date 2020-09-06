
I needed a simple distributed packet sniffer and most existing options I found were either too basic or too complex, so...

This tool will capture packets based on the filter you use when starting the agent on the selected interface and forward them to the archivist which is athe central process storing, indexing and serving the packets.

My other requirements was to be abelt to retrieve those packets as a pcap file for easy inspection in wireshark.



## Agent

They collect the packet data and store them in local, metadata are sent to the archivist


## Archivist

receives packet metadata from the agents and provide the api to query packets given a basic selector

## Archivist API

`/keys` returns a list of all the keys which are the source and destination ips.

`/download/<ip>` will produce and send a pcap file to the browser including all the packets captured by any of the agents matching this ip as source or destination.