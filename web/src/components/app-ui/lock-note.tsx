import * as React from "react";
import { LockKeyhole } from "lucide-react";

import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";

// LockNote is an accent-bordered callout with a padlock icon, a bold title and a
// muted body. Used for "this is locked / write-only" advisories the user cannot
// change (e.g. a write-only secret, the protected admin account). It mirrors the
// managed-role notice so locked controls read consistently across the app.
export function LockNote({
  title,
  children,
  className,
}: {
  title: string;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <Card size="sm" className={cn("mb-4 border-primary/70", className)}>
      <CardContent>
        <h2 className="mb-2 flex items-center gap-2 text-base font-semibold">
          <LockKeyhole className="size-4 text-primary" aria-hidden="true" />
          {title}
        </h2>
        <p className="m-0 text-sm leading-relaxed text-muted-foreground">{children}</p>
      </CardContent>
    </Card>
  );
}
