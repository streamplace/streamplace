// DID-document resolution helpers. Pure fetch, platform-agnostic.

// Expo inlines this exact property access; Vite bridges its VITE_PLC_URL here.
export const PLC_DIRECTORY_URL =
  process.env.EXPO_PUBLIC_PLC_URL || "https://plc.directory";

const plcDirectoryUrl = new URL(PLC_DIRECTORY_URL);

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
    // Match atproto's PLC resolver: DID documents live at the directory root.
    didDocUrl = new URL(`/${encodeURIComponent(did)}`, plcDirectoryUrl).href;
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

export async function getBlob(
  did: string,
  cid: string,
  didDoc?: DIDDocument,
): Promise<Blob> {
  const doc = didDoc || (await resolveDIDDocument(did));
  const pdsEndpoint = getPDSServiceEndpoint(doc);

  const blobUrl = `${pdsEndpoint}/xrpc/com.atproto.sync.getBlob?did=${did}&cid=${cid}`;

  const response = await fetch(blobUrl);
  if (!response.ok) {
    throw new Error(`Failed to fetch blob: ${response.status}`);
  }

  return response.blob();
}
