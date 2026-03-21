"use client";

import { useEffect } from "react";
import { useLexicalComposerContext } from "@lexical/react/LexicalComposerContext";
import { KEY_ENTER_COMMAND, COMMAND_PRIORITY_HIGH } from "lexical";

interface SubmitPluginProps {
  onSubmit: () => void;
}

export function SubmitPlugin({ onSubmit }: SubmitPluginProps) {
  const [editor] = useLexicalComposerContext();

  useEffect(() => {
    return editor.registerCommand(
      KEY_ENTER_COMMAND,
      (event: KeyboardEvent | null) => {
        if (event && !event.shiftKey && !event.ctrlKey && !event.metaKey) {
          event.preventDefault();
          onSubmit();
          return true;
        }
        return false;
      },
      COMMAND_PRIORITY_HIGH,
    );
  }, [editor, onSubmit]);

  return null;
}
