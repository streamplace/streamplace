import { createFileRoute, Link } from "@tanstack/react-router";
import { Globe, Info, Lock, Shield, User2, Video } from "lucide-react";
import { useTranslation } from "react-i18next";
import { EMPTY_LOGIN_SEARCH } from "../../lib/login-search";
import { useSession } from "../../lib/session";

export const Route = createFileRoute("/settings/")({
  component: SettingsIndex,
});

interface CategoryLinkProps {
  to: string;
  icon: React.ComponentType<{ size?: number; className?: string }>;
  label: string;
}

function CategoryLink({ to, icon: Icon, label }: CategoryLinkProps) {
  return (
    <Link
      to={to}
      className="flex items-center justify-between px-3 py-2.5 rounded-lg hover:bg-[var(--color-bg-elevated)] transition-colors group"
    >
      <div className="flex items-center gap-3">
        <Icon size={20} className="text-[var(--color-fg-muted)]" />
        <span className="text-sm">{label}</span>
      </div>
      <svg
        width="16"
        height="16"
        viewBox="0 0 16 16"
        fill="none"
        className="text-[var(--color-fg-muted)] opacity-0 group-hover:opacity-100 transition-opacity"
      >
        <path
          d="M6 3l5 5-5 5"
          stroke="currentColor"
          strokeWidth="1.5"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </svg>
    </Link>
  );
}

function SettingsIndex() {
  const { t } = useTranslation("settings");
  const { state, signOut } = useSession();

  const isLoggedIn = state.status === "authenticated";

  return (
    <div className="space-y-8">
      <header className="space-y-1">
        <h1 className="text-2xl font-semibold">{t("settings-title")}</h1>
      </header>

      {/* Account status */}
      <section>
        <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-elevated)] p-4 flex items-center justify-between">
          <div className="text-sm">
            {state.status === "authenticated" ? (
              <>
                Signed in as{" "}
                <span className="font-mono">{state.session.did}</span>
              </>
            ) : state.status === "loading" ? (
              <span className="text-[var(--color-fg-muted)]">Checking…</span>
            ) : (
              <span className="text-[var(--color-fg-muted)]">
                Not signed in
              </span>
            )}
          </div>
          {state.status === "authenticated" ? (
            <button
              type="button"
              onClick={() => {
                void signOut();
              }}
              className="h-9 px-4 rounded-md border border-[var(--color-border)] hover:border-[var(--color-border-strong)] text-sm"
            >
              {t("log-out")}
            </button>
          ) : (
            <Link
              to="/login"
              search={EMPTY_LOGIN_SEARCH}
              className="h-9 inline-flex items-center px-4 rounded-md bg-[var(--color-accent)] hover:bg-[var(--color-accent-hover)] text-[var(--color-accent-fg)] text-sm font-medium"
            >
              {t("sign-in")}
            </Link>
          )}
        </div>
      </section>

      {/* Category navigation */}
      {isLoggedIn && (
        <section>
          <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-elevated)] divide-y divide-[var(--color-border)]">
            <CategoryLink
              to="/settings/account"
              icon={User2}
              label={t("account")}
            />
            <CategoryLink
              to="/settings/streaming"
              icon={Video}
              label={t("streaming")}
            />
            <CategoryLink
              to="/settings/privacy"
              icon={Shield}
              label={t("privacy-security")}
            />
          </div>
        </section>
      )}

      <section>
        <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-elevated)] divide-y divide-[var(--color-border)]">
          <CategoryLink
            to="/settings/languages"
            icon={Globe}
            label={t("languages")}
          />
          <CategoryLink
            to="/settings/advanced"
            icon={Lock}
            label={t("advanced")}
          />
          <CategoryLink to="/settings/about" icon={Info} label={t("about")} />
        </div>
      </section>
    </div>
  );
}
