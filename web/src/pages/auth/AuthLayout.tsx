import { Network } from "lucide-react";
import type { ReactNode } from "react";
import { LanguageMenu } from "../../components/LanguageMenu";
import { ThemeMenu } from "../../components/ThemeMenu";

export function AuthLayout({ children }: { children: ReactNode }) {
  return (
    <main className="auth-layout flex min-h-dvh flex-col bg-background px-5 py-5 sm:px-8 sm:py-6">
      <header className="mx-auto flex w-full max-w-6xl items-center justify-between">
        <span className="flex items-center gap-2.5 text-sm font-semibold tracking-tight">
          <span className="flex size-8 items-center justify-center rounded-lg bg-accent-soft text-accent">
            <Network size={18} />
          </span>
          Headdeck
        </span>
        <div className="flex items-center gap-2">
          <LanguageMenu compact />
          <ThemeMenu />
        </div>
      </header>
      <div className="grid flex-1 place-items-center py-8">
        <div className="auth-card view-enter w-full max-w-100 overflow-hidden rounded-[1.25rem] border border-border bg-surface">
          <div className="flex min-w-0 items-center justify-center px-6 py-8 sm:px-8">
            <div className="w-full max-w-sm">{children}</div>
          </div>
        </div>
      </div>
    </main>
  );
}
