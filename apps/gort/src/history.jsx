
import config from "sk-config";
import createBrowserHistory from "history/lib/createBrowserHistory";
import { useRouterHistory } from "react-router";

const basePath = config.require("PUBLIC_GORT_BASE_PATH");

const browserHistory = useRouterHistory(createBrowserHistory)({
  basename: basePath,
});

export default browserHistory;
