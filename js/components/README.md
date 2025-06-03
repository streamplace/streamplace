# @streamplace/components

Heavily WIP but looks something like this:

```tsx
import {
  StreamplaceProvider,
  LivestreamProvider,
} from "@streamplace/components";

export function Provider() {
  <StreamplaceProvider url="https://stream.place" oauthSession={userSession}>
    {/* Everything inside of here can access that Streamplace node */}

    <LivestreamProvider src="example.bsky.social" /* or did:plc:xxxx */>
      {/* Everything in here has an active subscription to the livestream
        context via Websocket; things like chat data and stream title */}
      <App />
    </LivestreamProvider>
  </StreamplaceProvider>;
}

export function App() {
  const chat = useChat();
  return (
    <View>
      {chat.map((msg) => (
        <Text>
          @{msg.author.handle}: {msg.record.text}
        </Text>
      ))}
    </View>
  );
}
```
