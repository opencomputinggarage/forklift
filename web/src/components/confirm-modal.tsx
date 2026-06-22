import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

// In-app confirmation modal. The app never uses native dialogs (alert/confirm/
// prompt); all confirmations render through this component.
export function ConfirmModal({
  open,
  title,
  message,
  confirmLabel = "Confirm",
  danger,
  onConfirm,
  onCancel,
}: {
  open: boolean;
  title: string;
  message?: string;
  confirmLabel?: string;
  danger?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  if (!open) return null;
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm"
      onClick={onCancel}
    >
      <Card
        className="w-full max-w-[24rem] border-border bg-card shadow-2xl shadow-black/60"
        onClick={(e) => e.stopPropagation()}
      >
        <CardHeader>
          <CardTitle>{title}</CardTitle>
          {message && <CardDescription>{message}</CardDescription>}
        </CardHeader>
        <CardContent className="flex justify-end gap-2">
          <Button variant="outline" type="button" onClick={onCancel}>
            Cancel
          </Button>
          <Button
            variant={danger ? "destructive" : "default"}
            type="button"
            onClick={onConfirm}
          >
            {confirmLabel}
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
