// DID-document resolution helpers. Copied from
// js/components/src/utils/did.ts because the components package is
// React-Native-coupled and the web build doesn't depend on it. The
// implementation is platform-agnostic — pure fetch() — and is a good
// candidate to move into @streamplace/core in a future refactor.

export interface DIDDocument {
  id: string;
  service?: Array<{
    id: string;
    type?: string;
    serviceEndpoint?: string;
  }>;
  [key: string]: any;
}

export async function resolveDIDDocument(did: string): Promise<DIDDocument> {
  let didDocUrl: string;

  if (did.startsWith("did:web:")) {
    // For did:web, construct the URL directly
    const domain = did.replace("did:web:", "").replace(/:/g, "/");
    didDocUrl = `https://${domain}/.well-known/did.json`;
  } else if (did.startsWith("did:plc:")) {
    // For did:plc, use plc.directory
    didDocUrl = `https://plc.directory/${did}`;
  } else {
    throw new Error(`Unsupported DID method: ${did}`);
  }

  const response = await fetch(didDocUrl);
  if (!response.ok) {
    throw new Error(
      `Failed to resolve DID document for ${did}: ${response.status}`,
    );
  }

  return response.json();
}

export function getPDSServiceEndpoint(didDoc: DIDDocument): string {
  const pdsService = didDoc.service?.find((s) => s.id === "#atproto_pds");

  if (!pdsService?.serviceEndpoint) {
    throw new Error("No PDS service endpoint found in DID document");
  }

  return pdsService.serviceEndpoint;
}
