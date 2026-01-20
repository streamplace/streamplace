import { useEffect, useState } from "react";
import {
  atoms,
  Button,
  DialogFooter,
  ResponsiveDialog,
  Text,
  Textarea,
  useToast,
  View,
} from "../ui";

export interface UpdateStreamTitleDialogProps {
  livestream: any;
  streamerDID?: string;
  updateLivestream: (
    livestreamUri: string,
    title: string,
    streamerDID?: string,
  ) => Promise<any>;
  isLoading: boolean;
  onClose: () => void;
}

export function UpdateStreamTitleDialog({
  livestream,
  streamerDID,
  updateLivestream,
  isLoading,
  onClose,
}: UpdateStreamTitleDialogProps) {
  const [title, setTitle] = useState(livestream?.record?.title || "");
  const [error, setError] = useState<string | null>(null);
  const toast = useToast();

  useEffect(() => {
    if (livestream?.record?.title) {
      setTitle(livestream.record.title);
    }
  }, [livestream?.record?.title]);

  const handleUpdate = async () => {
    setError(null);

    if (!title.trim()) {
      setError("Please enter a stream title");
      return;
    }

    if (!livestream?.uri) {
      setError("No livestream found");
      return;
    }

    try {
      await updateLivestream(livestream.uri, title.trim(), streamerDID);
      toast.show(
        "Stream title updated",
        "The stream title has been successfully updated.",
        { duration: 3 },
      );
      onClose();
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to update stream title",
      );
    }
  };

  return (
    <ResponsiveDialog
      open={true}
      onOpenChange={(open) => {
        if (!open) {
          onClose();
          setError(null);
          setTitle(livestream?.record?.title || "");
        }
      }}
      title="Update Stream Title"
      description="Update the title of the livestream."
      size="md"
      dismissible={false}
    >
      <View style={[{ padding: 16, paddingBottom: 0 }]}>
        <View style={[{ marginBottom: 16 }]}>
          <Text
            style={[
              { color: atoms.colors.gray[300], fontSize: 13, marginBottom: 8 },
            ]}
          >
            Stream Title
          </Text>
          <Textarea
            value={title}
            onChangeText={(text) => {
              setTitle(text);
              setError(null);
            }}
            placeholder="Enter stream title..."
            maxLength={140}
            multiline
            style={[
              {
                padding: 12,
                borderRadius: 8,
                backgroundColor: atoms.colors.neutral[800],
                color: atoms.colors.white,
                borderWidth: 1,
                borderColor: atoms.colors.neutral[600],
                minHeight: 100,
                fontSize: 16,
              },
            ]}
          />
          <Text
            style={[
              { color: atoms.colors.gray[400], fontSize: 12, marginTop: 4 },
            ]}
          >
            {title.length}/140 characters
          </Text>
        </View>

        {error && (
          <View
            style={[
              {
                backgroundColor: atoms.colors.red[900],
                padding: 12,
                borderRadius: 8,
                borderWidth: 1,
                borderColor: atoms.colors.red[700],
                marginBottom: 16,
              },
            ]}
          >
            <Text style={[{ color: atoms.colors.red[400], fontSize: 13 }]}>
              {error}
            </Text>
          </View>
        )}
      </View>

      <DialogFooter>
        <Button
          width="min"
          variant="secondary"
          onPress={() => {
            onClose();
            setError(null);
            setTitle(livestream?.record?.title || "");
          }}
          disabled={isLoading}
        >
          <Text>Cancel</Text>
        </Button>
        <Button
          variant="primary"
          width="min"
          onPress={handleUpdate}
          disabled={isLoading || !title.trim()}
        >
          <Text>{isLoading ? "Updating..." : "Update Title"}</Text>
        </Button>
      </DialogFooter>
    </ResponsiveDialog>
  );
}
