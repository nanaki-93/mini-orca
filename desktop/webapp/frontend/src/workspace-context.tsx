import { createContext, useContext } from 'react';
import { workspace } from './workspace';

export const WorkspaceContext = createContext(workspace);
export const useWorkspace = () => useContext(WorkspaceContext);
