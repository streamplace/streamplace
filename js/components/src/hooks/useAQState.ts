import { useEffect, useState } from "react";
import storage from "../storage";

export function useAQState<T>(
  key: string,
  defaultValue: T,
): [T, (value: T) => void] {
  const [state, setState] = useState<T>(defaultValue);
  const [isLoaded, setIsLoaded] = useState(false);

  useEffect(() => {
    const loadFromStorage = async () => {
      try {
        const stored = await storage.getItem(key);
        if (stored !== null) {
          setState(JSON.parse(stored));
        }
      } catch (error) {
        console.error(`Failed to load ${key} from storage:`, error);
      } finally {
        setIsLoaded(true);
      }
    };
    loadFromStorage();
  }, [key]);

  const setStoredState = (value: T) => {
    setState(value);
    if (isLoaded) {
      storage.setItem(key, JSON.stringify(value)).catch((error) => {
        console.error(`Failed to save ${key} to storage:`, error);
      });
    }
  };

  return [state, setStoredState];
}
