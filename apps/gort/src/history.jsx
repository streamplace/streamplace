
import createBrowserHistory from "history/lib/createBrowserHistory";
import { useRouterHistory } from "react-router";

const browserHistory = useRouterHistory(createBrowserHistory)({
  basename: window.SK_PARAMS.BASE_URL,
});

export default browserHistory;
