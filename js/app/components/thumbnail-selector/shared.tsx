export interface ThumbnailSelectorProps {
  onThumbnailSelected: (blob: Blob | undefined) => void;
  thumbnailUrl?: string;
}
