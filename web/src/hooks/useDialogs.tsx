import { useState } from "react";
import { ConfirmDialog } from "../components/ConfirmDialog";
import type { ConfirmSpec } from "../components/ConfirmDialog";
import { FormDialog } from "../components/FormDialog";
import type { FormSpec } from "../components/FormDialog";

export function useDialogs() {
  const [form, setForm] = useState<FormSpec>();
  const [confirm, setConfirm] = useState<ConfirmSpec>();
  return {
    openForm: setForm,
    openConfirm: setConfirm,
    dialogs: (
      <>
        {form && <FormDialog form={form} onClose={() => setForm(undefined)} />}
        {confirm && <ConfirmDialog confirm={confirm} onClose={() => setConfirm(undefined)} />}
      </>
    ),
  };
}
