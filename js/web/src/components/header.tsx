import { Link } from "@tanstack/react-router";
import useAvatars from "../hooks/use-avatars";
import { EMPTY_LOGIN_SEARCH } from "../lib/login-search";
import { useSession } from "../lib/session";
import { useUserProfile } from "../lib/store/hooks";
import StreamplaceSvg from "./svg/streamplace-bw";
import { SidebarTrigger } from "./ui/sidebar";

export default function Header() {
  const { state } = useSession();
  // useUserProfile reads the authed user's profile from the slice's
  // `profiles` state, which the BlueskyProvider populates with
  // getProfile(did) on login. This is the reactive read for the
  // header — the avatar appears as soon as the slice finishes its
  // initial profile fetch.
  const userProfile = useUserProfile();
  // useAvatars is also called so the authed user's profile lands in
  // the batch cache (`profileCache`). Other consumers (e.g. a future
  // chat list, a multi-user grid) reading via useAvatars will dedupe
  // against this entry instead of refetching.
  useAvatars(state.status === "authenticated" ? [state.session.did] : []);

  const did = state.status === "authenticated" ? state.session.did : null;
  const avatar = userProfile?.avatar;
  const handle = userProfile?.handle;
  const displayName = userProfile?.displayName || handle;

  return (
    <>
      <header className="flex items-center gap-4 pt-2 pb-4 py-2 h-12 bg-sidebar">
        {/* fake sidebar left */}
        <div className="fixed left-4 top-3 z-50 flex items-center gap-2">
          <StreamplaceSvg className="w-6 h-6 invert-100" />
          <h1 className="text-lg">Streamplace</h1>
        </div>
        <div className="flex-1 flex items-center justify-end gap-4">
          <nav className="flex items-center gap-4">
            {did ? (
              <Link
                to="/$user"
                params={{ user: handle || did }}
                className="flex items-center gap-2 rounded-full hover:bg-[var(--color-bg-overlay)] transition-colors pl-1 pr-3 py-1"
                title={displayName ? `Signed in as @${handle}` : "Profile"}
                aria-label={displayName ? `Signed in as @${handle}` : "Profile"}
              >
                {avatar ? (
                  <img
                    src={avatar}
                    alt=""
                    className="w-7 h-7 rounded-full bg-[var(--color-bg)] object-cover"
                    onError={(e) => {
                      // Hide the img if the avatar URL 404s so the
                      // initial-letter fallback below shows through.
                      (e.currentTarget as HTMLImageElement).style.display =
                        "none";
                    }}
                  />
                ) : (
                  <div className="w-7 h-7 rounded-full bg-[var(--color-bg-overlay)] border border-[var(--color-border)] flex items-center justify-center text-xs font-medium text-[var(--color-fg-muted)]">
                    {(displayName?.[0] || handle?.[0] || "?").toUpperCase()}
                  </div>
                )}
                {displayName && (
                  <span className="text-sm font-medium text-[var(--color-fg)] hidden sm:inline">
                    @{handle}
                  </span>
                )}
              </Link>
            ) : (
              <Link
                to="/login"
                search={EMPTY_LOGIN_SEARCH}
                className="text-sm font-medium text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] transition-colors"
              >
                Log in
              </Link>
            )}
            <SidebarTrigger />
          </nav>
        </div>
      </header>
    </>
  );
}
