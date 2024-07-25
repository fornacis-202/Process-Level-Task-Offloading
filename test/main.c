#include <stdio.h>
#include <unistd.h>
#include <math.h>
#include <sys/types.h>
#include <unistd.h>
#include <stdlib.h>
#include <string.h>
#include <sys/socket.h>
#include <arpa/inet.h>
#include <iconv.h>

// Create a function to convert a string to utf-8
char* to_utf8(char* input) {
    // Create an iconv descriptor to convert from the current locale to utf-8
    iconv_t cd = iconv_open("UTF-8", "");
    if (cd == (iconv_t)-1) {
        perror("iconv_open failed");
        return NULL;
    }

    // Allocate a buffer to store the converted string
    size_t inlen = strlen(input); // The length of the input string
    size_t outlen = inlen * 4; // The maximum length of the output string (4 bytes per utf-8 character)
    char* output = malloc(outlen + 1); // Add one byte for the null terminator
    if (output == NULL) {
        perror("malloc failed");
        iconv_close(cd);
        return NULL;
    }

    // Convert the input string to the output buffer
    char* inptr = input; // A pointer to the input string
    char* outptr = output; // A pointer to the output buffer
    size_t n = iconv(cd, &inptr, &inlen, &outptr, &outlen); // The number of conversions performed
    if (n == (size_t)-1) {
        perror("iconv failed");
        free(output);
        iconv_close(cd);
        return NULL;
    }

    // Null terminate the output string
    *outptr = '\0';

    // Close the iconv descriptor
    iconv_close(cd);

    // Return the output string
    return output;
}



// Create a function to send a message to the server
void send_message(char* action, int pid) {
    // Create a socket
    int sock = socket(AF_INET, SOCK_STREAM, 0);
    if (sock == -1) {
        perror("Could not create socket");
        exit(1);
    }

    // Connect to the server
    struct sockaddr_in server;
    server.sin_addr.s_addr = inet_addr("127.0.0.1"); // Server IP address
    server.sin_family = AF_INET;
    server.sin_port = htons(9191); // Server port number

    if (connect(sock, (struct sockaddr *)&server, sizeof(server)) < 0) {
        printf("Connect failed\n");
        close(sock);
        return;
    }

    // Create a char array to store the formatted string
    char message[64]; // Adjust the size as needed

    // Fill the message with the action and the pid
    sprintf(message, "%s,%d", action, pid);

    // // Convert the message to utf-8
    // char* utf8_message = to_utf8(message);
    // if (utf8_message == NULL) {
    //     return;
    // }

    // Send the utf-8 message to the server
    if (send(sock, message, strlen(message), 0) < 0) {
        printf("Send failed\n");
        close(sock);
        return;
    }

    // Close the socket
    close(sock);
}

int main() {

    pid_t id;
    id = getpid();
    printf("pid is: %i\n",id);

    for (long long i = 0; i <= 10000000000; ++i) {
        double a = pow(4.6+i, 33.88 - i) + ((i+2)/(i+1));
        if (i % 100000000 ==0){
            printf("program is running in %lluth iteration. the result is: %f\n",i,a);
  
        }

        if (i == 100000000 || i == 10000000000){
            send_message("STP",id);
        }
    }
    sleep(1);

  return 0;
}



