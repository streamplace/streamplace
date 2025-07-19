import { ComAtprotoModerationCreateReport } from "@atproto/api";
import { useState } from "react";
import { usePDSAgent } from "../streamplace-store/xrpc";

export const useCreateReport = () => {
  const pdsAgent = usePDSAgent();
  const [error, setError] = useState<string | null>(null);

  const createReport = async (
    report: ComAtprotoModerationCreateReport.InputSchema,
  ) => {
    if (!pdsAgent) {
      setError("PDS Agent is not available");
      return;
    }

    const response = await pdsAgent.com.atproto.moderation.createReport(report);
    console.log(response);
    // setReport(response);
  };

  return {
    createReport,
    error,
  };
};
