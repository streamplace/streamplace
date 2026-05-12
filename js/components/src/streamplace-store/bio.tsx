import { Response } from "@atproto/api/dist/client/types/com/atproto/repo/getRecord";
import { PlaceStreamBioPage } from "streamplace";
import { leafletDocToBio } from "../lib/leaflet-to-bio";
import { getPDSServiceEndpoint, resolveDIDDocument } from "../utils/did";
import {
  getStreamplaceStoreFromContext,
  useDID,
  useStreamplaceStore,
} from "./streamplace-store";
import { usePDSAgent } from "./xrpc";

const BIO_COLLECTION = "place.stream.bio.page";
const BIO_RKEY = "self";
const LEAFLET_COLLECTION = "site.standard.document";

export function useBio() {
  return useStreamplaceStore((x) => x.bio);
}

export function useGetBio() {
  const did = useDID();
  const pdsAgent = usePDSAgent();
  const store = getStreamplaceStoreFromContext();

  return async () => {
    if (!did || !pdsAgent) {
      throw new Error("No DID or PDS agent");
    }
    let res: Response | undefined;
    try {
      res = await pdsAgent.com.atproto.repo.getRecord({
        repo: did,
        collection: BIO_COLLECTION,
        rkey: BIO_RKEY,
      });
    } catch (e) {
      if (
        e.error?.status === 400 &&
        e.error?.data?.error?.includes("Record not found")
      ) {
        store.setState({ bio: null });
        return null;
      }
      console.error("Failed to get bio record", e);
      return null;
    }

    if (res && PlaceStreamBioPage.isRecord(res.data.value)) {
      const bio = res.data.value as PlaceStreamBioPage.Record;
      store.setState({ bio });
      return bio;
    }
    return null;
  };
}

export function usePutBio() {
  const did = useDID();
  const pdsAgent = usePDSAgent();
  const store = getStreamplaceStoreFromContext();

  return async (bio: PlaceStreamBioPage.Record) => {
    if (!did || !pdsAgent) {
      throw new Error("No DID or PDS agent");
    }
    const res = await pdsAgent.com.atproto.repo.putRecord({
      repo: did,
      collection: BIO_COLLECTION,
      rkey: BIO_RKEY,
      record: bio,
    });
    if (!res.success) {
      throw new Error("Failed to put bio record");
    }
    store.setState({ bio });
  };
}

export interface ImportFromLeafletInput {
  /** AT-URI of a pub.leaflet.document record, or just an rkey to import from the user's own PDS. */
  source: string;
}

export interface ImportFromLeafletResult {
  bio: PlaceStreamBioPage.Record;
  warnings: string[];
}

export function useImportBioFromLeaflet() {
  const did = useDID();
  const pdsAgent = usePDSAgent();
  const putBio = usePutBio();

  return async (
    input: ImportFromLeafletInput,
  ): Promise<ImportFromLeafletResult> => {
    if (!did || !pdsAgent) {
      throw new Error("No DID or PDS agent");
    }
    const { repo, rkey } = resolveLeafletRef(input.source, did);

    // Fetch the leaflet document directly from the PDS to avoid the local proxy's
    // lexicon type checker, which rejects unknown $types like site.standard.document.
    const didDoc = await resolveDIDDocument(repo);
    const pdsEndpoint = getPDSServiceEndpoint(didDoc);
    const params = new URLSearchParams({
      repo,
      collection: LEAFLET_COLLECTION,
      rkey,
    });
    const docRes = await fetch(
      `${pdsEndpoint}/xrpc/com.atproto.repo.getRecord?${params}`,
    );
    if (!docRes.ok) {
      throw new Error(`Failed to fetch leaflet document: ${docRes.status}`);
    }
    const docJson = await docRes.json();

    const prior = await readExistingBio(pdsAgent, did);
    const importedFrom = `at://${repo}/${LEAFLET_COLLECTION}/${rkey}`;
    const { bio, warnings } = leafletDocToBio(docJson.value as object, {
      importedFrom,
      preserve: prior,
    }) as { bio: PlaceStreamBioPage.Record; warnings: string[] };
    await putBio(bio);
    return { bio, warnings };
  };
}

async function readExistingBio(
  pdsAgent: NonNullable<ReturnType<typeof usePDSAgent>>,
  did: string,
): Promise<PlaceStreamBioPage.Record | null> {
  try {
    const res = await pdsAgent.com.atproto.repo.getRecord({
      repo: did,
      collection: BIO_COLLECTION,
      rkey: BIO_RKEY,
    });
    if (res.success && PlaceStreamBioPage.isRecord(res.data.value)) {
      return res.data.value as PlaceStreamBioPage.Record;
    }
  } catch (e) {
    if (
      e.error?.status === 400 &&
      e.error?.data?.error?.includes("Record not found")
    ) {
      return null;
    }
    throw e;
  }
  return null;
}

function resolveLeafletRef(
  source: string,
  did: string,
): { repo: string; rkey: string } {
  const trimmed = source.trim();
  if (trimmed.startsWith("at://")) {
    const path = trimmed.slice("at://".length);
    const parts = path.split("/");
    if (parts.length !== 3 || parts[1] !== LEAFLET_COLLECTION) {
      throw new Error(
        `AT-URI must point to a ${LEAFLET_COLLECTION} record. Got: ${source}`,
      );
    }
    return { repo: parts[0], rkey: parts[2] };
  }
  // Bare rkey — assume the user's own PDS.
  if (trimmed.includes("/")) {
    throw new Error(
      "Source must be an AT-URI starting with 'at://' or a bare rkey.",
    );
  }
  return { repo: did, rkey: trimmed };
}
