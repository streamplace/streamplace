# @streamplace/components

```jsx
<StreamplaceProvider url="https://stream.place" pdsAgent={/* maybe? */}>
  {/* Everything inside of here can access that Streamplace node */}

  <LivestreamProvider src="scumb.ag">
    {/* Everything in here has an active subscription to the livestream
      context via Websocket; things like chat data and stream title */}

    <Player />
    <Chat />
    <ChatBox onSubmit={/* etc */} />
    {/* Open questions: how does an embedding app connect the logged in
    atproto user to this chat component? Do you inject an OAuth client
    or PDSAgent? */}
  </LivestreamProvider>
</StreamplaceProvider>
```
