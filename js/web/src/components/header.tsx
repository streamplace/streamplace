import StreamplaceSvg from "./svg/streamplace-bw";
import { SidebarTrigger } from "./ui/sidebar";

export default function Header() {
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
            <a
              href="/"
              className="text-sm font-medium text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] transition-colors"
            >
              Log in
            </a>
            <SidebarTrigger />
          </nav>
        </div>
      </header>
    </>
  );
}
