import { t } from "../lib/i18n";
import {
  Checkbox,
  Description,
  FieldError,
  Input,
  Label,
  ListBox,
  Select,
  Switch,
  TextField,
} from "@heroui/react";
import { ListEditor } from "./ListEditor";
import type { Field } from "../lib/types";

export function Fields({
  fields,
  values,
  onChange,
  disabled = false,
}: {
  fields: Field[];
  values: Record<string, string[]>;
  onChange: (name: string, value: string[]) => void;
  disabled?: boolean;
}) {
  return (
    <div className="space-y-4">
      {fields.map((field) => (
        <FieldInput
          key={field.name}
          field={field}
          value={values[field.name] ?? []}
          onChange={onChange}
          disabled={disabled}
        />
      ))}
    </div>
  );
}

function choiceLabel(field: Field, option: string) {
  return (
    field.choiceLabels?.[option] ??
    t(option === "true" ? "Enabled" : option === "false" ? "Disabled" : option)
  );
}

type FieldInputProps = {
  field: Field;
  value: string[];
  onChange: (name: string, value: string[]) => void;
  disabled: boolean;
};

function FieldInput({ field, value, onChange, disabled }: FieldInputProps) {
  if (field.control === "switch")
    return (
      <Switch
        isDisabled={disabled}
        isSelected={value[0] === "true"}
        onChange={(selected) => onChange(field.name, [String(selected)])}
        className="w-full py-1"
      >
        <Switch.Content className="min-h-9 w-full justify-between">
          <Label>{t(field.label)}</Label>
          <Switch.Control>
            <Switch.Thumb />
          </Switch.Control>
        </Switch.Content>
        {field.help && <Description>{t(field.help)}</Description>}
      </Switch>
    );
  if (field.multi && field.choices)
    return (
      <fieldset className="min-w-0 space-y-2" disabled={disabled}>
        <legend className="mb-2 text-sm font-medium">{t(field.label)}</legend>
        {field.choices.map((option) => (
          <Checkbox
            key={option}
            isDisabled={disabled}
            isSelected={value.includes(option)}
            onChange={(selected) =>
              onChange(
                field.name,
                selected ? [...value, option] : value.filter((item) => item !== option),
              )
            }
            className="w-full rounded-lg border border-border p-3"
          >
            <Checkbox.Content>
              <Checkbox.Control>
                <Checkbox.Indicator />
              </Checkbox.Control>
              <Label>{choiceLabel(field, option)}</Label>
            </Checkbox.Content>
          </Checkbox>
        ))}
        {field.help && <p className="text-sm text-muted">{t(field.help)}</p>}
      </fieldset>
    );
  if (field.choices?.length)
    return (
      <Select
        isDisabled={disabled}
        isRequired={field.required}
        value={value[0] ?? null}
        onChange={(next) => onChange(field.name, [String(next ?? "")])}
      >
        <Label>{t(field.label)}</Label>
        <Select.Trigger className="rounded-xl border border-border shadow-none">
          <Select.Value />
          <Select.Indicator />
        </Select.Trigger>
        <Select.Popover>
          <ListBox>
            {field.choices.map((option) => (
              <ListBox.Item id={option} key={option} textValue={choiceLabel(field, option)}>
                {choiceLabel(field, option)}
                <ListBox.ItemIndicator />
              </ListBox.Item>
            ))}
          </ListBox>
        </Select.Popover>
        {field.help && <Description>{t(field.help)}</Description>}
        <FieldError />
      </Select>
    );
  if (field.multi)
    return (
      <fieldset className="min-w-0 space-y-2" disabled={disabled}>
        <legend className="text-sm font-medium">{t(field.label)}</legend>
        <ListEditor
          label={t(field.label)}
          values={value}
          disabled={disabled}
          onChange={(next) => onChange(field.name, next)}
        />
        {field.help && <p className="text-sm text-muted">{t(field.help)}</p>}
      </fieldset>
    );
  return (
    <TextField
      isDisabled={disabled}
      isRequired={field.required}
      value={value[0] ?? ""}
      onChange={(next) => onChange(field.name, [next])}
    >
      <Label>{t(field.label)}</Label>
      <Input
        className="rounded-xl border border-border shadow-none"
        type={field.secret ? "password" : "text"}
        autoComplete={field.secret ? "new-password" : "off"}
      />
      {field.help && <Description>{t(field.help)}</Description>}
      <FieldError />
    </TextField>
  );
}
