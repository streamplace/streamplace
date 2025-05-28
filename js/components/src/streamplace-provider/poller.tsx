import { useEffect } from "react";
import { StreamplaceAgent } from "streamplace";
import { useStreamplaceStore } from "../streamplace-store";

export default function Poller({ children }: { children: React.ReactNode }) {
  const url = useStreamplaceStore((state) => state.url);
  const setLiveUsers = useStreamplaceStore((state) => state.setLiveUsers);

  console.log("poller", url);

  useEffect(() => {
    const agent = new StreamplaceAgent(url);
    const go = async () => {
      const res = await agent.place.stream.live.getLiveUsers();
      setLiveUsers(res.data.streams || []);
    };
    go();
    const handle = setInterval(go, 3000);
    return () => clearInterval(handle);
  }, [url]);

  return <>{children}</>;
}
