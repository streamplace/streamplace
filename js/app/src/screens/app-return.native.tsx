import { Redirect } from "components/aqlink";

export default function AppReturnScreen({ route }) {
  return <Redirect to={{ screen: "Home" }} />;
}
