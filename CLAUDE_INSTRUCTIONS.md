# Instructions for Your Claude Agents

Your Claude agents are encountering Docker conflicts. Here's how to fix it:

## For Each Running Agent

Send them this message to get their Docker environments working:

```
I see you're having Docker container conflicts. The setup script has been updated to fix this. Please run these commands to get your isolated environment working:

1. **Clean up and restart with the fixed script:**
   ```bash
   # Stop any conflicting containers
   docker-compose -f .docker/docker-compose.yaml down -v
   
   # Re-run the setup script (it's been fixed)
   ./.docker/setup-worktree-env.sh
   
   # Start your isolated services using the new script
   .docker/start-services.sh
   ```

2. **Verify it's working:**
   ```bash
   docker ps | grep rocketship-$(basename $(pwd))
   ```

3. **Use your isolated CLI:**
   ```bash
   .docker/docker-rocketship-local.sh --help
   ```

The key changes:
- Each worktree now gets a completely unique project name
- Containers are properly cleaned up before starting
- You now have simple start/stop scripts
- All networking conflicts are resolved

Your Docker environment should now work perfectly in isolation from the other agents.
```

## What Was Fixed

1. **Project Name Isolation**: Each worktree gets a unique Docker Compose project name
2. **Container Cleanup**: Automatic cleanup of conflicting containers
3. **Network Isolation**: Each worktree gets its own Docker network
4. **Simplified Commands**: New start-services.sh and stop-services.sh scripts
5. **Shell Compatibility**: Fixed the cd command issues

## Agent Commands

Each agent should now use:
- `.docker/start-services.sh` - Start their isolated environment
- `.docker/stop-services.sh` - Stop and clean up
- `.docker/docker-rocketship-local.sh` - Use the CLI

The agents should be able to work completely independently now!