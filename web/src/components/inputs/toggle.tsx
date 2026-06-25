import { Switch } from "@/components/ui/switch";

export function Toggle({ checked, onChange, disabled, label }: {
  checked: boolean;
  onChange: (v: boolean) => void;
  disabled?: boolean;
  label?: string;
}) {
  return (
    <label className="inline-flex items-center gap-2 text-sm">
      <Switch
        checked={checked}
        onCheckedChange={onChange}
        aria-label={label}
        disabled={disabled}
      />
      {label && <span>{label}</span>}
    </label>
  );
}
