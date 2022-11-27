package main;

import (
  "context"
  "encoding/json"
  "fmt"
  "log"
  "net/http"
  "os"

  "golang.org/x/oauth2"
  "golang.org/x/oauth2/google"
  "google.golang.org/api/drive/v3"
  "google.golang.org/api/option"

  "io"
  "errors"
  "time"
);

type Config struct {
  Files [][]string
};

func getClient(config *oauth2.Config) *http.Client { //U: Retrieve a token, saves the token, then returns the generated client.
  /* NOTE:
  *  The file token.json stores the user's access and refresh tokens, and is
  *  created automatically when the authorization flow completes for the first
  *  time.
  */
  tokFile := "token.json"; //TODO: Different users
  tok, err := tokenFromFile(tokFile);
  if err != nil { //A: First time
    tok = getTokenFromWeb(config);
    saveToken(tokFile, tok);
  }
  return config.Client(context.Background(), tok);
}

func getTokenFromWeb(config *oauth2.Config) *oauth2.Token { //U: Request a token from the web, then returns the retrieved token.
  authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline);
  fmt.Printf("Go to the following link in your browser then type the authorization code: \n%v\n", authURL);

  var authCode string
  if _, err := fmt.Scan(&authCode); err != nil {
    log.Fatalf("[getTokenFromWeb] Unable to read authorization code %v", err);
  }

  tok, err := config.Exchange(context.TODO(), authCode);
  if err != nil {
    log.Fatalf("[getTokenFromWeb] Unable to retrieve token from web %v", err);
  }

  return tok;
  //NOTE: Token is in the URL; TODO
}

func tokenFromFile(file string) (*oauth2.Token, error) { //U: Retrieves a token from a local file.
  f, err := os.Open(file);
  if err != nil {
    return nil, err;
  }
  defer f.Close();
  tok := &oauth2.Token{};
  err = json.NewDecoder(f).Decode(tok);
  return tok, err;
}

func saveToken(path string, token *oauth2.Token) { //U: Saves a token to a file path.
  fmt.Printf("Saving credential file to: %s\n", path);
  f, err := os.OpenFile(path, os.O_RDWR | os.O_CREATE | os.O_TRUNC, 0600);
  if err != nil {
    log.Fatalf("[saveToken] Unable to cache oauth token: %v", err);
  }
  defer f.Close();
  json.NewEncoder(f).Encode(token);
}

func getFile(drivePath string, service *drive.Service) (drive.File, error) {
  //r, err := service.Files.List().PageSize(10).Fields("nextPageToken, files(id, name)").Do();
  r, err := service.Files.List().Fields("files").Q(fmt.Sprintf("name = '%v' and 'root' in parents and trashed = false", drivePath)).Do(); //TODO: Check format string
  if err != nil {
    log.Printf("[getFileId] Unable to retrieve files: %v", err);
    return drive.File{}, errors.New("Unable to retrieve files");
  }

  if len(r.Files) == 1 { //TODO: Handle multiple files with the same name
    return *r.Files[0], nil; 
  }

  return drive.File{}, errors.New("File not found");
  //TODO: Cache
}

func downloadFile(localPath string, drivePath string, service *drive.Service) error {
  file, err := getFile(drivePath, service);
  if err != nil {
    log.Printf("[downloadFile] Unable to get file ID for \"%v\": %v", drivePath, err);
    return err;
  }
  
  r, err := service.Files.Get(file.Id).Download();
  if err != nil {
    log.Printf("[downloadFile] Unable to get file \"%v\" with id %v: %v", drivePath, err);
    return err;
  }
  defer r.Body.Close();

  if r.StatusCode == http.StatusOK {
    bodyBytes, err := io.ReadAll(r.Body);
    if err != nil {
      log.Printf("[downloadFile] Error reading file \"%v\": %v", drivePath, err);
      return err;
    }
    err = os.WriteFile(localPath, bodyBytes, 0644);
    if err != nil {
      log.Printf("[downloadFile] Error writing file \"%v\": %v", drivePath, err);
      return err;
    }
  }

  return nil;
  //TODO: Simpler?
}

func uploadFile(localPath string, drivePath string, service *drive.Service) error { //U: Uploads a local file to Drive
  file, err := os.Open(localPath);
  if err != nil {
    log.Printf("[uploadFile] Error opening \"%v\": %v", localPath, err);
    return errors.New(fmt.Sprintf("Couldn't open \"%v\"", localPath));
  }
  defer file.Close(); //TODO: https://gobyexample.com/defer

  _, err = service.Files.Create(&drive.File{Name: drivePath}).Media(file).Do();
  if err != nil {
    log.Printf("[uploadFile] Unable to create \"%v\": %v", drivePath, err);
  }

  return nil;
}

func syncFile(localPath string, drivePath string, service *drive.Service) error {
  log.Printf("[syncFile] localPath=\"%v\", drivePath=\"%v\"", localPath, drivePath);

  //A: Get local file and its date
  localFile, err := os.Stat(localPath);
  if err != nil {
    log.Printf("[syncFile] Error on getting local: %v", err);
    return err;
    //TODO: Error may mean it doesn't exist and we might want to download it
  }
  localDate := localFile.ModTime();

  //A: Get remote file and its date
  remoteFile, err := getFile(drivePath, service);
  if err != nil {
    log.Printf("[syncFile] Error on getting remote: %v", err);
    return err;
    //TODO: Error may mean it doesn't exist and we might want to upload
  }
  remoteDate, err := time.Parse(time.RFC3339, remoteFile.ModifiedTime);
  if err != nil {
    log.Printf("[syncFile] Error parsing remote time: %v", err);
    return err;
  }

  //TODO: Compare hashes

  if localDate.After(remoteDate) {
    return uploadFile(localPath, drivePath, service);
    //TODO: Replace rather than uploading
  } else if localDate.Before(remoteDate) {
    return downloadFile(localPath, drivePath, service);
  }

  //TODO: Config replacement policy / Force either

  return nil;
}

func syncAll(service *drive.Service) error {
  log.Print("[syncAll]");

  content, err := os.ReadFile("./config.json");
  if err != nil {
    log.Printf("[syncAll] Error reading config: %v", err);
    return errors.New("Error reading config");
  }

  var config Config;
  err = json.Unmarshal(content, &config);
  if err != nil {
    log.Printf("[syncAll] Error parsing config: %v", err);
    return errors.New("Error parsing config");
  }

  for _, file := range config.Files {
    err = syncFile(file[0], file[1], service);
    if err != nil {
      //TODO: Retry
    }
  }

  return nil;
}

func main() {
  ctx := context.Background();
  b, err := os.ReadFile("credentials.json");
  if err != nil {
    log.Fatalf("[main] Unable to read client secret file: %v", err);
  }

  //NOTE: Delete token.json if modifying these scopes
  config, err := google.ConfigFromJSON(b, drive.DriveScope); //A: Full access to Drive
  if err != nil {
    log.Fatalf("[main] Unable to parse client secret file to config: %v", err);
  }
  client := getClient(config);

  service, err := drive.NewService(ctx, option.WithHTTPClient(client));
  if err != nil {
    log.Fatalf("[main] Unable to retrieve Drive client: %v", err);
  }

  err = syncAll(service);
  if err != nil {
    log.Fatalf("[main] Error syncing: %v", err);
  }

  //localPath, drivePath := "test", "test";

  /*
  err = downloadFile(localPath, drivePath, service);
  if err != nil {
    log.Fatalf("[main] Unable to download file: %v", err);
  }
  */

  /*
  err = uploadFile("test", "test", service);
  if err != nil {
    log.Fatalf("[main] Unable to upload file: %v", err);
  }
  */
}

//TODO: Log vs user print
//TODO: Directories
